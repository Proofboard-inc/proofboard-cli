package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/proofboard/proofboard/internal/notifications"
)

func init() {
	notifications.VisibleSyncRunner = runInVisibleTerminal
}

// runInVisibleTerminal runs the Career Agent binary with the given args in
// workspace's directory inside a real, visible terminal window, instead of
// silently in the background. This backs actions explicitly chosen from the
// workspace-detection dialog/notification (e.g. "Sync Project"): the user
// asked for that action; they should see it actually run, not wonder whether
// anything happened.
func runInVisibleTerminal(ctx context.Context, workspace string, args ...string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	return openTerminalRunning(ctx, workspace, execPath, args)
}
