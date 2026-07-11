package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context, args []string) error {
	root := NewRootCommand(ctx, os.Stdout, os.Stderr)
	root.SetArgs(args)

	// Intercept execution on the very first run, even for bare commands
	_ = runFirstTimeSetup(ctx, root)

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("execute command: %w", err)
	}
	return nil
}

func NewRootCommand(ctx context.Context, out io.Writer, errOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "proofboard",
		Short:         "Local-first developer verification",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return runStartupUpdateChecks(cmd.Context(), cmd)
		},
	}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.AddCommand(
		newAuthCommand(ctx, out),
		newLinkCommand(ctx, out),
		newUnlinkCommand(ctx, out),
		newSyncCommand(ctx, out),
		newStatusCommand(ctx, out),
		newLogsCommand(ctx, out),
		newUpdateCommand(ctx, out),
		newUpdateDictionaryCommand(ctx, out),
		newConfigCommand(ctx, out),
		newCompletionCommand(),
		newInstallCommand(),
		newUninstallCommand(),
		newVersionCommand(ctx, out),
	)
	return cmd
}

func runStartupUpdateChecks(ctx context.Context, cmd *cobra.Command) error {
	name := cmd.Name()
	if name == "update" || name == "update-dictionary" || name == "help" || cmd.Parent() == nil {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	runCtx, err := loadRuntime(checkCtx)
	if err != nil {
		return nil
	}

	releases := api.NewReleaseClient(runCtx.config.ReleaseBaseURL)

	// 1. Check CLI Version
	latestCLI, err := releases.Latest(checkCtx, runCtx.config.LatestVersionPath)
	if err == nil && latestCLI.Version != "" && latestCLI.Version != version.Version {
		fmt.Fprintf(cmd.OutOrStdout(), "A new version of the Proofboard CLI is available. Run: proofboard update\n")
	}

	// 2. Check Dictionary Version
	stateData, err := runCtx.state.Load(checkCtx)
	if err == nil && stateData.AutoUpdateDictionary {
		localDict, err := dictionary.LoadDefault(checkCtx)
		if err == nil {
			url := fmt.Sprintf("%s%s", runCtx.config.APIBaseURL, runCtx.config.DictionaryPath)
			latestDict, err := releases.Latest(checkCtx, url)
			if err == nil && latestDict.Version != "" && latestDict.Version != localDict.Version {
				dir := filepath.Join(runCtx.homeDir, ".proofboard")
				if err := os.MkdirAll(dir, 0700); err == nil {
					tempPath := filepath.Join(dir, "dictionary.json.tmp")
					tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
					if err == nil {
						downloadErr := releases.Download(checkCtx, latestDict.URL, tempFile)
						tempFile.Close()
						if downloadErr == nil {
							validFile, err := os.Open(tempPath)
							if err == nil {
								downloadedDict, err := dictionary.Load(checkCtx, validFile)
								validFile.Close()
								if err == nil {
									if err := dictionary.Validate(downloadedDict); err == nil {
										targetPath := filepath.Join(dir, "dictionary.json")
										if err := os.Rename(tempPath, targetPath); err == nil {
											stateData.DictionaryVersion = latestDict.Version
											if err := runCtx.state.Save(checkCtx, stateData); err == nil {
												fmt.Fprintf(cmd.OutOrStdout(), "Dictionary updated successfully to version %s.\n", latestDict.Version)
											}
										}
									}
								}
							}
						}
						_ = os.Remove(tempPath)
					}
				}
			}
		}
	}

	return nil
}

func runFirstTimeSetup(ctx context.Context, cmd *cobra.Command) error {
	runCtx, err := loadRuntime(ctx)
	if err != nil {
		return nil
	}
	stateData, err := runCtx.state.Load(ctx)
	if err != nil {
		return nil
	}

	if stateData.FirstRunSetupComplete {
		return nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n--- Welcome to Proofboard! Let's get set up. ---\n")

	execPath, err := os.Executable()
	if err == nil {
		fmt.Fprintf(out, "Would you like to install proofboard permanently to your user PATH? (y/N): ")
		var answer1 string
		fmt.Scanln(&answer1)
		answer1 = strings.TrimSpace(strings.ToLower(answer1))
		if answer1 == "y" || answer1 == "yes" {
			if err := performInstall(out); err != nil {
				fmt.Fprintf(out, "Installation failed: %v\n", err)
			}
		}
	}

	shellPath := os.Getenv("SHELL")
	if shellPath != "" {
		shell := filepath.Base(shellPath)
		var rcFile string
		var installCmd string
		homeDir, err := os.UserHomeDir()
		if err == nil {
			switch shell {
			case "bash":
				rcFile = filepath.Join(homeDir, ".bashrc")
				installCmd = fmt.Sprintf("source <(%s completion bash)", execPath)
			case "zsh":
				rcFile = filepath.Join(homeDir, ".zshrc")
				installCmd = fmt.Sprintf("source <(%s completion zsh)", execPath)
			}

			if rcFile != "" {
				content, err := os.ReadFile(rcFile)
				if err != nil || !strings.Contains(string(content), installCmd) {
					fmt.Fprintf(out, "\nWould you like to install %s shell autocompletion? (y/N): ", shell)
					var answer2 string
					fmt.Scanln(&answer2)
					answer2 = strings.TrimSpace(strings.ToLower(answer2))
					if answer2 == "y" || answer2 == "yes" {
						f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
						if err != nil {
							fmt.Fprintf(out, "Failed to open %s: %v\n", rcFile, err)
						} else {
							f.WriteString(fmt.Sprintf("\n# Proofboard Autocompletion\n%s\n", installCmd))
							f.Close()
							fmt.Fprintf(out, "✓ Completions installed to %s. Please restart your terminal or run: source %s\n", rcFile, rcFile)
						}
					}
				}
			}
		}
	}

	stateData.FirstRunSetupComplete = true
	runCtx.state.Save(ctx, stateData)
	fmt.Fprintf(out, "--- Setup Complete ---\n\n")

	return nil
}

