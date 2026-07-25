package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

func newUninstallCommand() *cobra.Command {
	return newUninstallCommandWithAction(performUninstall)
}

func newUninstallCommandWithAction(action func(io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Proofboard Career Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return action(cmd.OutOrStdout())
		},
	}
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
	if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
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

func performUninstall(out io.Writer) error {
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

	if !removed {
		fmt.Fprintln(out, "No installed executable was found.")
		return nil
	}
	fmt.Fprintln(out, "✓ Executable removed.")
	return nil
}

func removeDirectoryFromPath(env installEnvironment, dir string) {
	if env.GOOS == "windows" {
		return
	}
	for _, target := range shellProfilePathTargets(env, dir) {
		_ = removeMarkedLine(target.Path, proofboardPathHeader, target.Line)
	}
}
