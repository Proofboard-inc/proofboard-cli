package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/logging"
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
		newNoticesCommand(ctx, out),
		newMilestoneCommand(ctx, out),
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
	if name == "update" || name == "update-dictionary" || name == "help" || name == "hook-maintain" || name == "notify" || name == "notify-activate" || name == "notices" || cmd.Parent() == nil {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	runCtx, err := loadRuntime(checkCtx)
	if err != nil {
		return nil
	}

	// 0. Self-heal shell startup hooks onto the current line — cheap, local
	// file I/O only (a small rc-file read + Contains check, no network), so
	// unlike the checks below this doesn't need throttling. Without this, an
	// existing install whose .zshrc/.bashrc/etc. still has an older hook
	// value (e.g. the backgrounded-and-fully-silenced `detect` line replaced
	// below) would never pick up the fix — hook-maintain only otherwise runs
	// once, at `proofboard install` time.
	_, _, _ = ensureShellDetectionHooks(checkCtx)

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
			switch {
			case updateErr == nil && result.Updated:
				stateData.DictionaryVersion = result.Version
				fmt.Fprintf(cmd.OutOrStdout(), "Dictionary updated successfully to version %s.\n", result.Version)
			case updateErr != nil && len(localDict.StackSignals) == 0:
				// The locally cached dictionary predates stack-signal data (or
				// was never successfully fetched at all) AND this refresh
				// attempt just failed — previously this branch was silent, so
				// tech-stack detection stayed permanently degraded (falling back
				// to a ~15-entry built-in table) with no visible sign anything
				// was wrong; a developer had no way to tell "up to date" from
				// "stuck on a years-old empty cache". Surface it (still
				// throttled to the same 6h cadence as the check itself).
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: could not refresh the category/tech-stack dictionary (%v).\nTech-stack detection will be incomplete until this succeeds — check your connection and try again.\n", updateErr)
				_ = logging.WriteSyncLog(runCtx.homeDir, "", "startup", "dictionary check", "failure", updateErr.Error())
			case updateErr != nil:
				_ = logging.WriteSyncLog(runCtx.homeDir, "", "startup", "dictionary check", "failure", updateErr.Error())
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
	case "notify", "notify-activate", "notices", "milestone-action", "hook-maintain":
		return true
	case "agent":
		return true
	default:
		return false
	}
}
