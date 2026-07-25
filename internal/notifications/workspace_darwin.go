//go:build darwin

package notifications

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	title, body, primary, secondary, tertiary := workspaceActionLabels(action.Kind)
	primaryKey, secondaryKey, tertiaryKey := workspaceActionKeys(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}

	var script string
	if secondary == "" {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(primary), strconv.Quote(primary))
	} else if tertiary == "" {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s, %s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(secondary), strconv.Quote(primary), strconv.Quote(primary))
	} else {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s, %s, %s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(tertiary), strconv.Quote(secondary), strconv.Quote(primary), strconv.Quote(primary))
	}
	// The dialog is raised by a background process, so it has to be asked for
	// explicitly or it never reaches the front. Giving up keeps a forgotten
	// dialog from holding the process open for the rest of the session.
	script = `tell application "System Events" to activate` + "\n" + script + ` with icon note giving up after 120`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out))+strings.ToLower(err.Error()), "user canceled") {
			return nil
		}
		// Reporting the failure is what makes a broken dialog diagnosable
		// from the sync log instead of looking like nothing happened.
		return fmt.Errorf("show workspace dialog: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.Contains(string(out), "button returned:"+primary) {
		return ActivateWorkspaceAction(ctx, primaryKey, action.Workspace, action.Target)
	}
	if strings.Contains(string(out), "button returned:"+secondary) && secondaryKey != "dismiss" {
		return ActivateWorkspaceAction(ctx, secondaryKey, action.Workspace, action.Target)
	}
	if strings.Contains(string(out), "button returned:"+tertiary) && tertiaryKey != "dismiss" {
		return ActivateWorkspaceAction(ctx, tertiaryKey, action.Workspace, action.Target)
	}
	return nil
}
