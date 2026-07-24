package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/proofboard/proofboard/internal/notifications"
	"github.com/spf13/cobra"
)

func newNotifyCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var kind string
	var workspace string
	var repoName string
	var target string

	cmd := &cobra.Command{
		Use:    "notify",
		Hidden: true,
		Short:  "Render a workspace notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "" {
				return nil
			}
			return notifications.ShowWorkspaceAction(ctx, notifications.WorkspaceAction{
				Kind:      kind,
				Workspace: workspace,
				RepoName:  repoName,
				Target:    target,
			})
		},
	}
	cmd.SetOut(out)
	cmd.Flags().StringVar(&kind, "kind", "", "workspace state kind")
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace path")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "repository name")
	cmd.Flags().StringVar(&target, "target", "", "notification action target")
	return cmd
}

func newNotifyActivateCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "notify-activate",
		Hidden: true,
		Short:  "Handle notification activation callbacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := ""
			if len(args) > 0 {
				raw = args[0]
			}
			if raw == "" {
				return nil
			}
			kind, workspace, target, err := notifications.ParseWorkspaceTargetActionURI(raw)
			if err != nil {
				return fmt.Errorf("parse notification activation: %w", err)
			}
			return notifications.ActivateWorkspaceAction(ctx, kind, workspace, target)
		},
	}
	cmd.SetOut(out)
	return cmd
}
