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
	title, body, primary, secondary := workspaceActionLabels(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}

	script := fmt.Sprintf(`display dialog %s with title %s buttons {%s, %s} default button %s cancel button %s`,
		strconv.Quote(body),
		strconv.Quote(title),
		strconv.Quote(primary),
		strconv.Quote(secondary),
		strconv.Quote(primary),
		strconv.Quote(secondary),
	)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out))+strings.ToLower(err.Error()), "user canceled") {
			return nil
		}
		return nil
	}
	if strings.Contains(string(out), "button returned:"+primary) {
		return ActivateWorkspaceAction(ctx, "sync", action.Workspace)
	}
	return nil
}
