//go:build windows

package notifications

import (
	"context"
	"fmt"

	toast "git.sr.ht/~jackmordaunt/go-toast"
)

func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	_ = ctx
	title, body, primary, secondary := workspaceActionLabels(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}

	n := toast.Notification{
		AppID:               "Proofboard",
		Title:               title,
		Body:                body,
		Duration:            toast.Long,
		ActivationType:      toast.Protocol,
		ActivationArguments: WorkspaceActionURI("sync", action.Workspace),
		Actions: []toast.Action{
			{Type: toast.Protocol, Content: primary, Arguments: WorkspaceActionURI("sync", action.Workspace)},
			{Type: toast.Protocol, Content: secondary, Arguments: WorkspaceActionURI("dismiss", action.Workspace)},
		},
	}
	if err := n.Push(); err != nil {
		return nil
	}
	return nil
}
