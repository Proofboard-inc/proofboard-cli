package commands

import (
	"context"
	"fmt"
	"io"
	"net/url"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/config"
	"github.com/spf13/cobra"
)

// newMilestoneCommand groups the three actions offered whenever the Career
// Agent surfaces a detected milestone (see printMilestonesReady in
// runtime.go). Milestone detection is a plain terminal message with no
// buttons, so these are real, documented subcommands: "review"/"publish"/
// "skip" read far better as command names than a single generic
// `milestone-action <verb> <id>`.
func newMilestoneCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Review, publish, or skip a Career Agent-detected milestone",
	}
	cmd.SetOut(out)
	cmd.AddCommand(
		newMilestoneReviewCommand(ctx, out),
		newMilestonePublishCommand(ctx, out),
		newMilestoneSkipCommand(ctx, out),
	)
	return cmd
}

func newMilestoneReviewCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review <bundle-id>",
		Short: "Open a detected milestone in your Proofboard dashboard to inspect it before publishing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dashboardURL := milestoneDashboardURL(ctx, args[0])
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", dashboardURL); err != nil {
				return err
			}
			return pbauth.OpenBrowser(ctx, dashboardURL)
		},
	}
	cmd.SetOut(out)
	return cmd
}

func newMilestonePublishCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <bundle-id>",
		Short: "Publish a detected milestone as-is",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := executeMilestoneBundleAction(ctx, out, "publish", args[0]); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Milestone published.")
			return err
		},
	}
	cmd.SetOut(out)
	return cmd
}

func newMilestoneSkipCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skip <bundle-id>",
		Short: "Dismiss a detected milestone without publishing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := executeMilestoneBundleAction(ctx, out, "ignore", args[0]); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Milestone skipped.")
			return err
		},
	}
	cmd.SetOut(out)
	return cmd
}

// milestoneDashboardURL resolves the CLI's actual configured frontend URL
// (respecting PROOFBOARD_APP_BASE_URL), falling back to the built-in
// default if config can't load for any reason. Mirrors appBaseURL in
// internal/notifications/workspace.go.
func milestoneDashboardURL(ctx context.Context, bundleID string) string {
	base := config.DefaultAppBaseURL
	if cfg, err := config.Load(ctx); err == nil && cfg.AppBaseURL != "" {
		base = cfg.AppBaseURL
	}
	return base + "/dashboard?milestoneBundle=" + url.QueryEscape(bundleID)
}
