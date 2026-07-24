//go:build windows

package notifications

import (
	"context"
	"fmt"

	toast "git.sr.ht/~jackmordaunt/go-toast"
)

func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	_ = ctx
	title, body, primary, secondary, tertiary := workspaceActionLabels(action.Kind)
	primaryKey, secondaryKey, tertiaryKey := workspaceActionKeys(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}
	actions := []toast.Action{{Type: toast.Protocol, Content: primary, Arguments: WorkspaceTargetActionURI(primaryKey, action.Workspace, action.Target)}}
	if secondary != "" {
		actions = append(actions, toast.Action{Type: toast.Protocol, Content: secondary, Arguments: WorkspaceTargetActionURI(secondaryKey, action.Workspace, action.Target)})
	}
	if tertiary != "" {
		actions = append(actions, toast.Action{Type: toast.Protocol, Content: tertiary, Arguments: WorkspaceTargetActionURI(tertiaryKey, action.Workspace, action.Target)})
	}

	n := toast.Notification{
		AppID:               "Proofboard Career Agent",
		Title:               title,
		Body:                body,
		Duration:            toast.Long,
		ActivationType:      toast.Protocol,
		ActivationArguments: WorkspaceTargetActionURI(primaryKey, action.Workspace, action.Target),
		Actions:             actions,
	}
	if err := n.Push(); err != nil {
		return nil
	}
	return nil
}
