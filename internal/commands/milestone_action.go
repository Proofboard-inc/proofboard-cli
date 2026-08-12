package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// newMilestoneActionCommand stays hidden: it's now reached only via the
// notification-activation callback path (notify-activate ->
// ActivateWorkspaceAction -> runMilestoneAction in
// internal/notifications/workspace.go), which remains available for the
// `proofboard notify --kind milestone` manual/test path even though nothing
// automatically raises an OS-level milestone notification anymore. The
// user-facing equivalent is the `proofboard milestone publish|skip`
// subcommands in milestone.go, which call the same executeMilestoneBundleAction
// below.
func newMilestoneActionCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "milestone-action publish|ignore bundle-id",
		Short:  "Handle a Career Agent milestone action",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMilestoneBundleAction(ctx, out, args[0], args[1])
		},
	}
	cmd.SetOut(out)
	return cmd
}

// executeMilestoneBundleAction publishes ("publish") or declines ("ignore")
// a milestone bundle. Shared by the hidden milestone-action command above
// and the friendly `proofboard milestone publish|skip` subcommands in
// milestone.go.
func executeMilestoneBundleAction(ctx context.Context, out io.Writer, action, bundleID string) error {
	runtime, err := loadRuntime(ctx)
	if err != nil {
		return fmt.Errorf("milestone action: %w", err)
	}
	if _, err := loadOrAuthCredentials(ctx, out, runtime); err != nil {
		return fmt.Errorf("milestone action authenticate: %w", err)
	}
	return retryAfterAuth(ctx, out, "milestone action", func() error {
		credentials, loadErr := runtime.credentials.Load(ctx)
		if loadErr != nil {
			return fmt.Errorf("load milestone credentials: %w", loadErr)
		}
		switch action {
		case "publish":
			return runtime.api.ApproveMilestoneBundle(ctx, credentials.Token, bundleID)
		case "ignore":
			return runtime.api.DeclineMilestoneBundle(ctx, credentials.Token, bundleID)
		default:
			return fmt.Errorf("unsupported milestone action %q", action)
		}
	})
}
