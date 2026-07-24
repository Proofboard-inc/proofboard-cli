package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/logging"
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

			result, err := detection.Inspect(ctx, runtime.homeDir, workspace, editor)
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
				_ = launchWorkspaceNotification(ctx, runtime, result)
			} else if result.Action == detection.ActionSync {
				_ = launchWorkspaceSync(ctx, result.WorkspacePath)
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

func launchWorkspaceNotification(ctx context.Context, runtime runtimeContext, result detection.Result) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "notify",
		"--kind", string(result.Action),
		"--workspace", result.WorkspacePath,
		"--repo-name", result.RepoName,
	)
	return cmd.Start()
}
