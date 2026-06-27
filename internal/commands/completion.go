package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate the autocompletion script or auto-install it",
		Long: `Generate the autocompletion script for proofboard for the specified shell.
If no shell is specified, it will attempt to auto-detect your shell and offer to install it automatically.`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				shellPath := os.Getenv("SHELL")
				if shellPath == "" {
					if runtime.GOOS == "windows" {
						shellPath = "powershell"
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "Could not auto-detect shell (SHELL env var is empty). Please specify: bash, zsh, fish, or powershell.")
						return nil
					}
				}
				shell := filepath.Base(shellPath)
				fmt.Fprintf(cmd.OutOrStdout(), "Auto-detected shell: %s\n", shell)

				homeDir, err := os.UserHomeDir()
				if err != nil {
					return err
				}

				execPath, err := os.Executable()
				if err != nil {
					execPath = "proofboard"
				}

				var rcFile string
				var installCmd string
				switch shell {
				case "bash":
					rcFile = filepath.Join(homeDir, ".bashrc")
					installCmd = fmt.Sprintf("source <(%s completion bash)", execPath)
				case "zsh":
					rcFile = filepath.Join(homeDir, ".zshrc")
					installCmd = fmt.Sprintf("source <(%s completion zsh)", execPath)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "Auto-installation is not currently supported for %s. You can manually generate it using: proofboard completion %s\n", shell, shell)
					return nil
				}

				content, err := os.ReadFile(rcFile)
				if err == nil && strings.Contains(string(content), installCmd) {
					fmt.Fprintf(cmd.OutOrStdout(), "Completions are already installed in %s\n", rcFile)
					return nil
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Would you like to auto-install %s shell autocompletion? (y/N): ", shell)
				var answer string
				fmt.Scanln(&answer)
				answer = strings.TrimSpace(strings.ToLower(answer))

				if answer == "y" || answer == "yes" {
					f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
					if err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Failed to open %s: %v\n", rcFile, err)
						return err
					}
					defer f.Close()
					f.WriteString(fmt.Sprintf("\n# Proofboard Autocompletion\n%s\n", installCmd))
					fmt.Fprintf(cmd.OutOrStdout(), "✓ Completions installed to %s. Please restart your terminal or run: source %s\n", rcFile, rcFile)
				}
				return nil
			}

			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell type: %q", args[0])
			}
		},
	}
}
