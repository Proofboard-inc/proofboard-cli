package commands

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// newNoticesCommand surfaces any unread Proofboard notifications (sync
// complete, milestones ready to review, etc.) directly in the terminal. It is
// invoked synchronously from the shell startup hook (unlike `detect`, which
// stays backgrounded/silent) so the output is actually visible the next time
// a terminal window opens: the same "subtle line on shell startup" pattern
// as a venv auto-activation hook, just for Proofboard status instead of a
// Python environment.
func newNoticesCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "notices",
		Hidden: true,
		Short:  "Print any unread Proofboard notifications once",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return nil
			}
			// Bounded the same way the startup version/dictionary checks are:
			// a slow network must not noticeably delay a new shell prompt.
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			surfaceUnreadNotifications(checkCtx, cmd.OutOrStdout(), runtime)
			return nil
		},
	}
	cmd.SetOut(out)
	return cmd
}
