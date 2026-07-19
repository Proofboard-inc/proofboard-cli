//go:build !linux && !windows && !darwin

package notifications

import "context"

func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	return nil
}
