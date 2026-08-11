package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/spf13/cobra"
)

func newAuthCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var rotateKey bool
	var switchAccount bool
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
			// device-code network round trip. Previously this ran unconditionally,
			// so a developer who was already connected but had a flaky/offline
			// network got a hard "auth login: send request: ... timeout" error for
			// a command that had nothing to actually do. --switch (or --rotate-key,
			// which needs a live round trip to register the new key) bypasses this.
			if !switchAccount && !rotateKey {
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
	cmd.AddCommand(newAuthLogoutCommand(ctx, out))
	return cmd
}

func newAuthLogoutCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove local Proofboard credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("auth logout: %w", err)
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
			_, err = fmt.Fprintln(out, "Proofboard local credentials removed. This device is logged out.")
			return err
		},
	}
}
