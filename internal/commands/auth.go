package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/spf13/cobra"
)

func newAuthCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var rotateKey bool
	var switchAccount bool
	var forceReauth bool
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Connect the Proofboard Career Agent",
		Long: "Connects this machine to your Proofboard account via a device-code sign-in.\n" +
			"If you're already authenticated, this reports your current connection status\n" +
			"instead of starting a new sign-in — use --switch to connect a different account.",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			// Already authenticated: report status instead of forcing a fresh
			// device-code network round trip, so a developer who is already
			// connected but has a flaky/offline network isn't shown a hard
			// "auth login: send request: ... timeout" error for a command that
			// has nothing to actually do. --switch (or --rotate-key, which needs
			// a live round trip to register the new key) bypasses this.
			// forceReauth is what the automatic reconnect uses. Without it
			// the short-circuit below fires on the very credentials the
			// server just rejected: a sync gets a 401, the reconnect runs
			// this command, it reports "already authenticated" because a
			// token is present, and the sync retries with the same dead
			// token. Sign-in could never actually recover a stale session.
			if !switchAccount && !rotateKey && !forceReauth {
				if existing, loadErr := runtime.credentials.Load(ctx); loadErr == nil &&
					existing.Token != "" && !credentialsCompletelyExpired(existing) {
					name := existing.Username
					if name == "" {
						name = "Proofboard user"
					}
					_, printErr := fmt.Fprintf(out,
						"Already authenticated as %s. Proofboard Career Agent is connected.\n"+
							"Run `proofboard auth --switch` to connect a different account, or `proofboard auth logout` first.\n",
						name,
					)
					return printErr
				}
			}
			emailHash, emailErr := authEmailHash(ctx)
			if emailErr != nil {
				if existing, loadErr := runtime.credentials.Load(ctx); loadErr == nil && existing.EmailHash != "" {
					emailHash = existing.EmailHash
				} else {
					emailHash = ""
				}
			}
			service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
			credentials, err := service.Login(ctx, emailHash)
			if err != nil {
				return fmt.Errorf("auth login: %w", err)
			}
			// Authentication is not complete until this installation can
			// prove ownership of a registered signing key. Never report a
			// connected state that can only produce unsigned payloads.
			keyStore := pbauth.NewDeviceKeyStore(runtime.homeDir)
			if deviceKey, keyErr := keyStore.Ensure(ctx, runtime.api, credentials.Token, rotateKey); keyErr != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, "", "auth", "register device key", "warning", keyErr.Error())
				return fmt.Errorf("register device signing key: %w", keyErr)
			} else {
				credentials.DeviceKeyID = deviceKey.DeviceKeyID
				if err := runtime.credentials.Save(ctx, credentials); err != nil {
					return fmt.Errorf("persist device key id: %w", err)
				}
			}
			if current, loadErr := runtime.state.Load(ctx); loadErr == nil {
				current.AuthReconnectPrompted = false
				current.AuthReconnectPromptedAt = time.Time{}
				current.AuthLoggedOut = false
				_ = runtime.state.Save(ctx, current)
			}
			name := credentials.Username
			if name == "" {
				name = "Proofboard user"
			}
			_, err = fmt.Fprintf(out, "Authenticated as %s. Proofboard Career Agent is connected and projects will sync automatically.\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&rotateKey, "rotate-key", false, "generate a new device signing key")
	cmd.Flags().BoolVar(&switchAccount, "switch", false, "connect a different Proofboard account, even if one is already connected")
	cmd.Flags().BoolVar(&forceReauth, "force", false, "sign in again even if credentials are already present")
	_ = cmd.Flags().MarkHidden("force")
	cmd.AddCommand(newAuthLogoutCommand(ctx, out))
	return cmd
}

// newAuthLogoutCommand is `proofboard auth logout`, which sits with the rest
// of the sign-in commands. newLogoutCommand below exposes the same thing at
// the top level, because that is where people look for it first.
func newAuthLogoutCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign this device out of Proofboard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(ctx, cmd.OutOrStdout())
		},
	}
}

// newLogoutCommand is `proofboard logout`. Signing out is not something
// people go looking for under a sub-command, so it is available directly as
// well; both run exactly the same code rather than one shelling out to the
// other or the two drifting apart.
func newLogoutCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Sign this device out of Proofboard",
		Long: "Removes this device's Proofboard credentials and stops the background\n" +
			"agent from syncing until you sign in again. Identical to\n" +
			"`proofboard auth logout`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(ctx, cmd.OutOrStdout())
		},
	}
	cmd.SetOut(out)
	return cmd
}

// runLogout removes the stored credentials and records that the sign-out was
// deliberate, so the agent does not immediately prompt to reconnect as though
// the session had merely expired.
func runLogout(ctx context.Context, out io.Writer) error {
	runtime, err := loadRuntime(ctx)
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	if err := runtime.credentials.Delete(ctx); err != nil {
		return err
	}
	current, stateErr := runtime.state.Load(ctx)
	if stateErr == nil {
		current.AuthLoggedOut = true
		current.AuthReconnectPrompted = false
		current.AuthReconnectPromptedAt = time.Time{}
		if err := runtime.state.Save(ctx, current); err != nil {
			return fmt.Errorf("persist logged-out state: %w", err)
		}
	}
	// Signing out clears what this device holds about the account, not only
	// the token: the activity log records what was synced and when, the
	// device signing key identifies this machine to the service, and the
	// cached notification state belongs to the account that just left.
	// Leaving those behind means "logged out" only described the token.
	removed := clearLocalAccountData(runtime.homeDir)

	if _, err := fmt.Fprintln(out, "Proofboard local credentials removed. This device is logged out."); err != nil {
		return err
	}
	if removed > 0 {
		_, err = fmt.Fprintf(out, "Activity log and device signing key cleared (%d file(s)).\n", removed)
		return err
	}
	return nil
}

// clearLocalAccountData removes the per-account files the Career Agent keeps
// on this machine and reports how many it deleted. Best effort throughout: a
// file that cannot be removed must not abort the sign-out, because a failed
// sign-out that leaves the credentials in place is worse than one that leaves
// a log behind.
//
// The dictionary is deliberately left: it is public reference data, identical
// for everyone, and re-downloading it on the next sign-in would be pure waste.
func clearLocalAccountData(homeDir string) int {
	dir := filepath.Join(homeDir, ".proofboard")
	removed := 0
	for _, name := range []string{
		"sync.log",   // what this account synced, and when
		"sync.log.1", // the rotated previous log
		"auto-update.log",
		"device.key", // identifies this machine to the service
	} {
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			removed++
		}
	}
	return removed
}
