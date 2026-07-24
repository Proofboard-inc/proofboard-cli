package notifications

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	statestore "github.com/proofboard/proofboard/internal/state"
)

type WorkspaceAction struct {
	Kind      string
	Workspace string
	RepoName  string
	Target    string
}

func WorkspaceActionURI(kind, workspace string) string {
	return WorkspaceTargetActionURI(kind, workspace, "")
}

func WorkspaceTargetActionURI(kind, workspace, target string) string {
	v := url.Values{}
	v.Set("kind", kind)
	v.Set("workspace", workspace)
	if target != "" {
		v.Set("target", target)
	}
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

func ParseWorkspaceTargetActionURI(raw string) (kind string, workspace string, target string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("parse notification uri: %w", err)
	}
	if u.Scheme != "proofboard" || u.Host != "notify-action" {
		return "", "", "", fmt.Errorf("unsupported notification uri: %s", raw)
	}
	q := u.Query()
	return q.Get("kind"), q.Get("workspace"), q.Get("target"), nil
}

func ShowWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	return showWorkspaceAction(ctx, action)
}

func ActivateWorkspaceAction(ctx context.Context, kind, workspace string, target ...string) error {
	actionTarget := ""
	if len(target) > 0 {
		actionTarget = target[0]
	}
	switch kind {
	case "", "dismiss":
		return nil
	case "suppress":
		return suppressWorkspace(ctx, workspace)
	case "sync", "link":
		return runWorkspaceSync(ctx, workspace)
	case "reconnect":
		return runAgentAuth(ctx)
	case "review", "publish":
		if kind == "publish" && actionTarget != "" {
			return runMilestoneAction(ctx, "publish", actionTarget)
		}
		dashboardURL := "https://proofboard.io/dashboard"
		if actionTarget != "" {
			dashboardURL += "?milestoneBundle=" + url.QueryEscape(actionTarget)
		}
		return pbauth.OpenBrowser(ctx, dashboardURL)
	case "ignore":
		if actionTarget == "" {
			return nil
		}
		return runMilestoneAction(ctx, "ignore", actionTarget)
	default:
		return fmt.Errorf("unsupported workspace action %q", kind)
	}
}

func runMilestoneAction(ctx context.Context, action, target string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "milestone-action", action, target)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func suppressWorkspace(ctx context.Context, workspace string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	store := statestore.NewStore(homeDir)
	current, err := store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load workspace suppression state: %w", err)
	}
	current, err = statestore.AddWorkspaceSuppression(current, workspace)
	if err != nil {
		return err
	}
	if err := store.Save(ctx, current); err != nil {
		return fmt.Errorf("save workspace suppression state: %w", err)
	}
	return nil
}

func runWorkspaceSync(ctx context.Context, workspace string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "sync", "--agent")
	cmd.Dir = workspace
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func runAgentAuth(ctx context.Context) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "auth")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func workspaceActionLabels(kind string) (title string, body string, primary string, secondary string, tertiary string) {
	switch kind {
	case "sync":
		return "Project needs sync", "Proofboard can capture the latest work privately on this machine.", "Sync Project", "Not Now", "Never Ask Again"
	case "link":
		return "New repository detected", "Would you like Proofboard to track this project? No proprietary source code leaves your computer.", "Sync Project", "Not Now", "Never Ask Again"
	case "reconnect":
		return "Your Proofboard session has expired", "Reconnect to resume private background synchronization.", "Reconnect", "", ""
	case "milestone":
		return "Milestone detected", "Review this engineering milestone before publishing it.", "Review", "Publish", "Ignore"
	default:
		return "Proofboard Career Agent", "Open Proofboard to continue.", "Open", "Not Now", "Never Ask Again"
	}
}

func workspaceActionKeys(kind string) (primary string, secondary string, tertiary string) {
	switch kind {
	case "reconnect":
		return "reconnect", "", ""
	case "milestone":
		return "review", "publish", "ignore"
	default:
		return "sync", "dismiss", "suppress"
	}
}
