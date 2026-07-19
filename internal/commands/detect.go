package commands

import (
	"context"
	"encoding/json"
	"io"
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
		Use:   "detect",
		Short: "Inspect an opened workspace and surface link or sync actions",
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

func isNotGitRepoError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a git repository") || strings.Contains(msg, "rev-parse --show-toplevel")
}
