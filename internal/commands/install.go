package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install proofboard permanently to your PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			return performInstall(cmd.OutOrStdout())
		},
	}
}

func newUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove proofboard from your PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			return performUninstall(cmd.OutOrStdout())
		},
	}
}

func performInstall(out io.Writer) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, _ = filepath.Abs(execPath)

	var destDir string
	var destFile string

	if runtime.GOOS != "windows" {
		destDir = "/usr/local/bin"
		destFile = "/usr/local/bin/proofboard"

		if execPath == destFile {
			fmt.Fprintln(out, "Proofboard is already installed system-wide.")
			return nil
		}

		fmt.Fprintf(out, "Installing to %s...\n", destFile)
		input, err := os.ReadFile(execPath)
		if err != nil {
			return err
		}

		err = os.WriteFile(destFile, input, 0755)
		if err != nil && os.IsPermission(err) {
			fmt.Fprintln(out, "Permission denied. Attempting to install using sudo (you may be prompted for your password)...")
			cmd := exec.Command("sudo", "cp", execPath, destFile)
			cmd.Stdin = os.Stdin
			cmd.Stdout = out
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install using sudo: %w", err)
			}
			cmd = exec.Command("sudo", "chmod", "755", destFile)
			cmd.Run()
		} else if err != nil {
			return err
		}
		fmt.Fprintln(out, "✓ Global installation successful.")
		return nil
	} else {
		destDir = filepath.Join(os.Getenv("ProgramFiles"), "Proofboard")
		destFile = filepath.Join(destDir, "proofboard.exe")

		if execPath == destFile {
			fmt.Fprintln(out, "Proofboard is already installed system-wide.")
			return nil
		}

		fmt.Fprintf(out, "Installing to %s...\n", destFile)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied. Please run this command as an Administrator")
			}
			return err
		}

		input, err := os.ReadFile(execPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destFile, input, 0755); err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied. Please run this command as an Administrator")
			}
			return err
		}

		psCmd := fmt.Sprintf(`$path = [Environment]::GetEnvironmentVariable('Path', 'Machine'); if ($path -notmatch '%s') { [Environment]::SetEnvironmentVariable('Path', $path + ';%s', 'Machine') }`, strings.ReplaceAll(destDir, `\`, `\\`), strings.ReplaceAll(destDir, `\`, `\\`))
		cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(out, "Warning: Failed to add to System PATH. Please run as Administrator or add %s to your PATH manually.\n", destDir)
		} else {
			fmt.Fprintln(out, "Added to System PATH.")
		}
		fmt.Fprintln(out, "✓ Global installation successful.")
		return nil
	}
}

func performUninstall(out io.Writer) error {
	var destDir string
	var destFile string

	if runtime.GOOS != "windows" {
		destFile = "/usr/local/bin/proofboard"

		fmt.Fprintf(out, "Removing %s...\n", destFile)
		err := os.Remove(destFile)
		if err != nil && os.IsPermission(err) {
			fmt.Fprintln(out, "Permission denied. Attempting to remove using sudo...")
			cmd := exec.Command("sudo", "rm", "-f", destFile)
			cmd.Stdin = os.Stdin
			cmd.Stdout = out
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to uninstall using sudo: %w", err)
			}
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Fprintln(out, "✓ Executable removed globally.")
		return nil
	} else {
		destDir = filepath.Join(os.Getenv("ProgramFiles"), "Proofboard")
		destFile = filepath.Join(destDir, "proofboard.exe")

		fmt.Fprintf(out, "Removing %s...\n", destFile)
		if err := os.Remove(destFile); err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied. Please run this command as an Administrator to uninstall")
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		fmt.Fprintln(out, "✓ Executable removed. You may need to manually remove the Proofboard directory from your System PATH environment variable.")
		return nil
	}
}
