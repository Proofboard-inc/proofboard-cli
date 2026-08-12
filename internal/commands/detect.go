package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/logging"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/style"
	"github.com/spf13/cobra"
)

func newDetectCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var workspace string
	var editor string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:    "detect",
		Short:  "Inspect an opened workspace and surface Career Agent actions",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return nil
			}
			if workspace == "" {
				workspace = runtime.workingDir
			}

			// detect now runs synchronously from shell startup (see
			// shell_hooks.go) rather than backgrounded, so its output is
			// actually visible in the new terminal instead of being discarded.
			// Inspect only does local git plumbing, no network, but this bound
			// still protects shell startup from ever hanging on a pathological
			// repo — same pattern as the other startup checks in root.go.
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			result, err := detection.Inspect(checkCtx, runtime.homeDir, workspace, editor)
			if err != nil {
				if isNotGitRepoError(err) {
					return nil
				}
				_ = logging.WriteSyncLog(runtime.homeDir, "", "detect", "failure", "inspect workspace", err.Error())
				return nil
			}

			reason := result.Reason
			if reason == "" {
				reason = string(result.Action)
			}
			_ = logging.WriteSyncLog(runtime.homeDir, result.RepoHash, "detect", string(result.Action), reason, "")
			if result.Action == detection.ActionLink {
				// Record the prompt before showing it so a second terminal
				// opening at the same moment cannot raise it twice.
				if err := recordWorkspacePrompt(ctx, runtime, result.WorkspacePath); err != nil {
					_ = logging.WriteSyncLog(runtime.homeDir, result.RepoHash, "detect", "failure", "record workspace prompt", err.Error())
					return nil
				}
				// `detect` runs inside the very terminal the shell-open hook
				// just backgrounded it from, so an OS-level popup is
				// unnecessary machinery here — print straight to that
				// terminal instead. (See printLinkDetected.) Skipped in
				// --json mode, which must emit nothing but the JSON payload.
				if !jsonOutput {
					printLinkDetected(cmd.OutOrStdout())
				}
			} else if result.Action == detection.ActionSync {
				_ = launchWorkspaceSync(ctx, result.WorkspacePath)
				if !jsonOutput {
					printSyncNeeded(cmd.OutOrStdout())
				}
			}

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				if err := encoder.Encode(result); err != nil {
					return nil
				}
			}

			return nil
		},
	}
	cmd.SetOut(out)

	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root to inspect")
	cmd.Flags().StringVar(&editor, "editor", "", "editor or IDE name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable detection output")
	return cmd
}

func recordWorkspacePrompt(ctx context.Context, runtime runtimeContext, workspace string) error {
	store := statestore.NewStore(runtime.homeDir)
	current, err := store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	updated, err := statestore.RecordWorkspacePrompt(current, workspace, time.Now())
	if err != nil {
		return fmt.Errorf("record workspace prompt: %w", err)
	}
	return store.Save(ctx, updated)
}

func launchWorkspaceSync(ctx context.Context, workspace string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "sync", "--incremental", "--agent")
	cmd.Dir = workspace
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return startDetachedCommand(cmd)
}

func isNotGitRepoError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a git repository") || strings.Contains(msg, "rev-parse --show-toplevel")
}

// printLinkDetected replaces the OS-level "Project detected" notification
// for the plain `detect` path. The repo/org name is deliberately left out —
// it's not needed to make the message actionable, and detect.go otherwise
// keeps that identity out of anything it prints or logs locally (see the
// sync.log entry above, which only ever carries RepoHash).
func printLinkDetected(out io.Writer) {
	fmt.Fprintf(out, "%s New repository detected\n", style.Success(out, "✓"))
	fmt.Fprintln(out, "  Run `proofboard link` to add this to your career record.")
	fmt.Fprintln(out, "  Run `proofboard link --dismiss` if you don't want to be asked about this workspace again.")
}

// printSyncNeeded replaces the OS-level "Project needs sync" notification.
// The sync itself is already kicked off in the background by
// launchWorkspaceSync above (unchanged) — this is purely informational.
func printSyncNeeded(out io.Writer) {
	fmt.Fprintf(out, "%s Project needs sync\n", style.Success(out, "✓"))
	fmt.Fprintln(out, "  Syncing the latest commits privately in the background — no action needed.")
}
