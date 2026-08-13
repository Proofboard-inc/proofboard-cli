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
				// `detect` runs inside the very terminal the shell-open hook
				// just backgrounded it from, so an OS-level popup is
				// unnecessary machinery here — print straight to that
				// terminal instead. (See printLinkDetected.) Skipped in
				// --json mode, which must emit nothing but the JSON payload.
				// Nothing is recorded here on the "ask again later" paths —
				// the workspace keeps being offered on every future terminal
				// until the developer either links it or explicitly says
				// "never" (same effect as `proofboard link --dismiss`,
				// which is the only thing that silences it for good).
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
//
// This is deliberately print-only — it must NEVER read from stdin. `detect`
// runs synchronously from the shell startup hook (see shell_hooks.go), so
// blocking here waiting for keyboard input would hold up the shell prompt
// itself on every single terminal opened in an unconnected repo — the exact
// failure mode the legacy backgrounded/silenced hook line was trying (badly)
// to work around. The "y/n" happens after the fact instead, as two ordinary
// commands the developer runs on their own time: `proofboard sync` is the
// yes — it authenticates, connects, and syncs in one step, the full flow
// `link` alone doesn't cover on its own — and `proofboard link --dismiss` is
// the "never ask again for this project." Neither answer is required before
// the terminal is usable.
func printLinkDetected(out io.Writer) {
	fmt.Fprintf(out, "%s %s %s\n",
		style.Success(out, "✓"),
		style.Brand(out, "Proofboard"),
		style.Heading(out, "— New repository detected. Sync it?"))
	printCommandHint(out, "proofboard sync", "yes — connect and sync")
	printCommandHint(out, "proofboard link --dismiss", "never ask again for this project")
	fmt.Fprintln(out, style.Muted(out, "  (do nothing to be asked again next time)"))
}

// printCommandHint renders one "command   description" row, keeping the
// description column aligned regardless of the command's own ANSI escape
// codes (padding is computed from the plain command text, then the colored
// version is substituted in — coloring first and padding after would count
// invisible escape bytes toward the column width and break alignment).
func printCommandHint(out io.Writer, command, description string) {
	const column = 30
	pad := column - len(command)
	if pad < 2 {
		pad = 2
	}
	fmt.Fprintf(out, "    %s%s%s\n",
		style.Accent(out, command),
		strings.Repeat(" ", pad),
		style.Muted(out, description))
}

// printSyncNeeded replaces the OS-level "Project needs sync" notification.
// The sync itself is already kicked off in the background by
// launchWorkspaceSync above (unchanged) — this is purely informational.
func printSyncNeeded(out io.Writer) {
	fmt.Fprintf(out, "%s %s %s\n",
		style.Success(out, "✓"),
		style.Brand(out, "Proofboard"),
		style.Heading(out, "— project needs sync"))
	fmt.Fprintln(out, style.Muted(out, "  Syncing the latest commits privately in the background — no action needed."))
}
