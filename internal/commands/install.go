package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/spf13/cobra"
)

func newInstallCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "install",
		Short: "Install and start Proofboard Career Agent",
		Long: `Install and start Proofboard Career Agent.

The Career Agent installs into your own account and needs no administrator
access. Use --system to install it for every account on the machine, which
does require administrator access.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			systemWide, err := cmd.Flags().GetBool("system")
			if err != nil {
				return err
			}
			return installTo(cmd.OutOrStdout(), systemWide)
		},
	}
	command.Flags().Bool("system", false, "Install for every account on this machine (requires administrator access)")
	return command
}

func newInstallCommandWithAction(action func(io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start Proofboard Career Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return action(cmd.OutOrStdout())
		},
	}
}

func newUninstallCommand(ctx context.Context) *cobra.Command {
	return newUninstallCommandWithAction(ctx, performUninstall)
}

func newUninstallCommandWithAction(ctx context.Context, action func(context.Context, io.Writer) error) *cobra.Command {
	var assumeYes bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Proofboard Career Agent",
		Long: `Remove Proofboard Career Agent.

This removes the installed executable, stops and unregisters the background
service, strips every shell hook this CLI has written (PATH export,
workspace detection, autocompletion) from every shell profile it may have
touched, and signs this device out — clearing the locally stored session
credentials and this device's signing key. Use --yes to skip the
confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !assumeYes {
				confirmed, err := confirmUninstall(cmd.InOrStdin(), cmd.OutOrStdout())
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Uninstall cancelled. Nothing was removed.")
					return nil
				}
			}
			return action(ctx, cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Skip the confirmation prompt")
	return command
}

// confirmUninstall asks before doing something this destructive and hard to
// undo from inside the CLI itself (there is no "proofboard reinstall my old
// session back"): it removes local files, kills the background service, and
// signs the device out of the account entirely. Defaults to "no" on an empty
// or unreadable answer, same convention as promptPersonalProjectConfirm's
// "no" branch and every other destructive-by-default prompt in this CLI.
func confirmUninstall(in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprintln(out, "This will remove Proofboard Career Agent, delete its shell hooks from your")
	fmt.Fprintln(out, "shell profiles, and sign this device out (clearing local credentials and")
	fmt.Fprintln(out, "this device's signing key).")
	fmt.Fprint(out, "Continue? [y/N]: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		// No input available (e.g. piped from /dev/null) — do not silently
		// proceed with a destructive action; treat it the same as "no".
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// performInstall installs the running executable for the current user.
func performInstall(out io.Writer) error {
	return installTo(out, false)
}

func installTo(out io.Writer, systemWide bool) error {
	env, err := currentInstallEnvironment()
	if err != nil {
		return err
	}
	location := resolveInstallLocation(env, systemWide)

	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, _ = filepath.Abs(execPath)

	if execPath == location.Executable {
		fmt.Fprintf(out, "Proofboard Career Agent is already installed at %s.\n", location.Executable)
	} else {
		if err := copyExecutable(execPath, location, out); err != nil {
			return err
		}
		fmt.Fprintf(out, "Installed to %s.\n", location.Executable)
	}

	if !location.SystemWide {
		if err := ensureDirectoryOnPath(env, location.Dir, out); err != nil {
			return err
		}
	}
	// Workspace detection runs from a shell startup hook. Installing is an
	// explicit action by the person running it, so this is where the hook is
	// put in place; ordinary commands never touch shell profiles.
	//
	// This must pass location.Executable explicitly rather than calling the
	// self-healing ensureShellDetectionHooks(ctx), which resolves the hook
	// binary from os.Executable() — the *currently running* process. At this
	// exact point that is still whatever copied the executable into place (a
	// temp download location from the install script, or a dev build run
	// manually from a repo checkout), not the destination path just resolved
	// above. Hooks written against the wrong one would point at a path that
	// may not exist once the installer's temp files are cleaned up.
	if _, _, err := ensureShellDetectionHooksForBinary(context.Background(), location.Executable); err != nil {
		fmt.Fprintf(out, "Warning: Failed to enable workspace detection: %v\n", err)
	}
	if err := installAgentService(location.Executable, out); err != nil {
		return fmt.Errorf("register Career Agent: %w", err)
	}
	fmt.Fprintln(out, "✓ Proofboard Career Agent installed and started.")
	return nil
}

func copyExecutable(execPath string, location installLocation, out io.Writer) error {
	if err := os.MkdirAll(location.Dir, 0o755); err != nil {
		if os.IsPermission(err) {
			return permissionError(location)
		}
		return err
	}
	content, err := os.ReadFile(execPath)
	if err != nil {
		return err
	}

	// Replacing a running executable fails on some systems, so the new one is
	// staged next to the destination and moved into place.
	staged := location.Executable + ".new"
	if err := os.WriteFile(staged, content, 0o755); err != nil {
		if os.IsPermission(err) {
			return permissionError(location)
		}
		return err
	}
	if err := os.Rename(staged, location.Executable); err != nil {
		_ = os.Remove(staged)
		if os.IsPermission(err) {
			return permissionError(location)
		}
		return err
	}
	if err := os.Chmod(location.Executable, 0o755); err != nil {
		fmt.Fprintf(out, "Warning: Failed to mark %s executable: %v\n", location.Executable, err)
	}
	return nil
}

func permissionError(location installLocation) error {
	if location.SystemWide {
		return fmt.Errorf("permission denied writing to %s. Re-run with administrator access, or drop --system to install into your own account without it", location.Dir)
	}
	return fmt.Errorf("permission denied writing to %s", location.Dir)
}

func performUninstall(ctx context.Context, out io.Writer) error {
	env, err := currentInstallEnvironment()
	if err != nil {
		return err
	}

	if err := uninstallAgentService(out); err != nil {
		return fmt.Errorf("unregister Career Agent: %w", err)
	}
	_ = unregisterProtocolHandler()

	removed := false
	for _, location := range knownInstallLocations(env) {
		if _, statErr := os.Stat(location.Executable); statErr != nil {
			continue
		}
		fmt.Fprintf(out, "Removing %s...\n", location.Executable)
		if err := os.Remove(location.Executable); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(out, "Warning: %s needs administrator access to remove; skipping.\n", location.Executable)
				continue
			}
			if !os.IsNotExist(err) {
				return err
			}
			continue
		}
		removed = true
		if !location.SystemWide {
			removeDirectoryFromPath(env, location.Dir)
		}
	}

	// Strip every shell-hook block this CLI may have written (PATH export,
	// workspace-detection hooks, autocompletion), not just the PATH line for
	// the location(s) actually found above — run unconditionally so a manual
	// removal of the binary, or an install on a since-changed $SHELL, still
	// gets fully cleaned up.
	removeShellHookBlocks(env)

	// Uninstalling must also sign the device out. Previously it did not:
	// the executable and its shell hooks were gone, but the session
	// credentials and device signing key stayed in the OS keychain (or the
	// file fallback) untouched, so reinstalling reported the same account
	// as still authenticated even though "uninstall" had just run. Mirrors
	// runLogout's best-effort semantics — a keychain this process cannot
	// reach (headless Linux, containers, SSH, a locked login keychain) must
	// not abort the rest of the uninstall.
	clearUninstallAuthState(ctx, env.HomeDir, out)

	if !removed {
		fmt.Fprintln(out, "No installed executable was found.")
		return nil
	}
	fmt.Fprintln(out, "✓ Executable removed.")
	return nil
}

// clearUninstallAuthState signs this device out as part of uninstall: it
// clears the stored session credentials, this device's signing key (both
// checked in the OS keychain and the on-disk fallback), and the local
// account files (activity log, device key file if not already handled by
// the keychain path). Best-effort throughout, same rationale as runLogout —
// a failure here must not abort the rest of uninstall, which has already
// started removing the executable and shell hooks by the time this runs.
func clearUninstallAuthState(ctx context.Context, homeDir string, out io.Writer) {
	_ = pbauth.NewCredentialStore(homeDir).Delete(ctx)
	_ = pbauth.NewDeviceKeyStore(homeDir).Delete(ctx)
	clearLocalAccountData(homeDir)
	fmt.Fprintln(out, "✓ Signed out and local credentials cleared.")
}

func removeDirectoryFromPath(env installEnvironment, dir string) {
	if env.GOOS == "windows" {
		return
	}
	for _, target := range shellProfilePathTargets(env, dir) {
		_ = removeMarkedLine(target.Path, proofboardPathHeader, target.Line)
	}
}
