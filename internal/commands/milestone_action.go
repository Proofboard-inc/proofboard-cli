package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newMilestoneActionCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "milestone-action publish|ignore bundle-id",
		Short:  "Handle a Career Agent milestone action",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("milestone action: %w", err)
			}
			if _, err := loadOrAuthCredentials(ctx, out, runtime); err != nil {
				return fmt.Errorf("milestone action authenticate: %w", err)
			}
			action := args[0]
			bundleID := args[1]
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
		},
	}
	cmd.SetOut(out)
	return cmd
}
