package commands

import (
	"context"
	"fmt"
	"io"
	"os"
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
		Short:         "Proofboard Career Agent — private, local-first engineering proof",
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
		newAgentCommand(ctx, out),
		newDetectCommand(ctx, out),
		newNotifyCommand(ctx, out),
		newNotifyActivateCommand(ctx, out),
		newMilestoneActionCommand(ctx, out),
		newShellHookMaintenanceCommand(ctx, out),
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
	if os.Getenv("PROOFBOARD_DISABLE_STARTUP_CHECKS") == "1" {
		return nil
	}
	name := cmd.Name()
	if name == "update" || name == "update-dictionary" || name == "help" || name == "hook-maintain" || name == "notify" || name == "notify-activate" || cmd.Parent() == nil {
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
		fmt.Fprintf(cmd.OutOrStdout(), "A new version of Proofboard Career Agent is available. Run: proofboard update\n")
	}

	// 2. Check Dictionary Version — throttled to at most once per 6h.
	// This function runs on every command via PersistentPreRunE, and `sync`
	// (fired by post-merge/post-pull git hooks on every commit/pull) is not
	// in the exclusion list above — so without a cadence gate this was
	// hitting the dictionary endpoint on every single commit/pull, risking
	// 429s. The gate covers the ATTEMPT, not just successful updates: a
	// failed check is throttled too, so a flaky/down release server can't
	// turn into a check-on-every-command retry storm either.
	stateData, err := runCtx.state.Load(checkCtx)
	if err == nil && stateData.AutoUpdateDictionary &&
		(stateData.LastDictionaryUpdateCheck.IsZero() || time.Since(stateData.LastDictionaryUpdateCheck) >= 6*time.Hour) {
		if localDict, loadErr := dictionary.LoadDefault(checkCtx); loadErr == nil {
			dictionaryURL := fmt.Sprintf("%s%s", runCtx.config.APIBaseURL, runCtx.config.DictionaryPath)
			result, updateErr := dictionary.Update(checkCtx, runCtx.homeDir, dictionaryURL, localDict)
			stateData.LastDictionaryUpdateCheck = time.Now().UTC()
			if updateErr == nil && result.Updated {
				stateData.DictionaryVersion = result.Version
				fmt.Fprintf(cmd.OutOrStdout(), "Dictionary updated successfully to version %s.\n", result.Version)
			}
			_ = runCtx.state.Save(checkCtx, stateData)
		}
	}

	notifyAuthExpiry(checkCtx, cmd.OutOrStdout(), runCtx)
	surfaceUnreadNotifications(checkCtx, cmd.OutOrStdout(), runCtx)

	return nil
}

func isInternalCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "notify", "notify-activate", "milestone-action", "hook-maintain":
		return true
	case "agent":
		return true
	default:
		return false
	}
}
