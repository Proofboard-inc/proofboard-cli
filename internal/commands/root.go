package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context, args []string) error {
	root := NewRootCommand(ctx, os.Stdout, os.Stderr)
	root.SetArgs(args)
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
			latestDict, err := releases.Latest(checkCtx, runCtx.config.LatestDictionaryPath)
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

