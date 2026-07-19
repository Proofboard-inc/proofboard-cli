package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/notifications"
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
				return fmt.Errorf("detect: %w", err)
			}
			if workspace == "" {
				workspace = runtime.workingDir
			}
			result, err := detection.Inspect(ctx, runtime.homeDir, workspace, editor)
			if err != nil {
				return fmt.Errorf("inspect workspace: %w", err)
			}

			if jsonOutput {
				data, err := result.Marshal()
				if err != nil {
					return fmt.Errorf("marshal detection result: %w", err)
				}
				_, err = fmt.Fprintln(out, string(data))
				return err
			}

			switch result.Action {
			case detection.ActionLink:
				notifications.Dispatch(out, notifications.NewProjectDetected(result.RepoName))
			case detection.ActionSync:
				notifications.Dispatch(out, notifications.ProjectSyncNeeded(result.RepoName))
			default:
				return nil
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root to inspect")
	cmd.Flags().StringVar(&editor, "editor", "", "editor or IDE name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON instead of terminal output")
	return cmd
}

func promptForNewProjectDetection(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintln(out, "Add this project to your proofboard?")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  y   Sync this project")
		fmt.Fprintln(out, "  n   Not this project")
		fmt.Fprintln(out, "  x   Never ask for this workspace")
		fmt.Fprint(out, "Choose [y/n/x]: ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "n"
		}
		choice := strings.ToLower(strings.TrimSpace(line))
		switch choice {
		case "y", "n", "x":
			return choice
		}
	}
}
