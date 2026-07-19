package notifications

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
)

type WorkspaceAction struct {
	Kind      string
	Workspace string
	RepoName  string
}

func WorkspaceActionURI(kind, workspace string) string {
	v := url.Values{}
	v.Set("kind", kind)
	v.Set("workspace", workspace)
	return "proofboard://notify-action?" + v.Encode()
}

func ParseWorkspaceActionURI(raw string) (kind string, workspace string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse notification uri: %w", err)
	}
	if u.Scheme != "proofboard" || u.Host != "notify-action" {
		return "", "", fmt.Errorf("unsupported notification uri: %s", raw)
	}
	q := u.Query()
	return q.Get("kind"), q.Get("workspace"), nil
}

func ShowWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	return showWorkspaceAction(ctx, action)
}

func ActivateWorkspaceAction(ctx context.Context, kind, workspace string) error {
	if kind == "" || kind == "dismiss" {
		return nil
	}
	return runWorkspaceSync(ctx, workspace)
}

func runWorkspaceSync(ctx context.Context, workspace string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "sync")
	cmd.Dir = workspace
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func workspaceActionLabels(kind string) (title string, body string, primary string, secondary string) {
	switch kind {
	case "sync":
		return "Project needs sync", "Open Proofboard sync to capture the latest work.", "Sync now", "Dismiss"
	case "link":
		return "New project detected", "Add this workspace to Proofboard.", "Add to Proofboard", "Dismiss"
	default:
		return "Proofboard", "Open Proofboard to continue.", "Open", "Dismiss"
	}
}
