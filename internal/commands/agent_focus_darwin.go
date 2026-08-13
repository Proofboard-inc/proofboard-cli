//go:build darwin

package commands

import (
	"context"
	"os/exec"
	"strings"
)

// isWorkspaceFocused reports whether the given workspace's IDE window is the
// one currently frontmost on screen. This is what stops the agent from
// notifying about every open-but-backgrounded editor window: only the one
// the user is actually looking at should prompt. workspaceCount is the
// number of IDE workspaces the agent is currently tracking this tick (see
// discoverIDEWorkspaces); it bounds the fail-open fallback below.
func isWorkspaceFocused(ctx context.Context, workspace string, ideNames []string, workspaceCount int) bool {
	script := `tell application "System Events"
	set frontApp to name of first process whose frontmost is true
	set winTitle to ""
	try
		set winTitle to name of front window of (first process whose frontmost is true)
	end try
end tell
return frontApp & "|" & winTitle`
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		// Can't determine focus (osascript unavailable, Accessibility
		// permission denied, etc.). Fail open (fire) only when this is the
		// only workspace the agent is tracking right now: there's nothing
		// else it could be confused with. With more than one tracked
		// workspace we can't tell whether THIS one is actually focused, so
		// suppress rather than risk notifying about every background window.
		return workspaceCount <= 1
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	frontApp := parts[0]
	windowTitle := ""
	if len(parts) > 1 {
		windowTitle = strings.ToLower(parts[1])
	}
	if !matchesFrontmostIDEName(frontApp, ideNames) {
		return false
	}
	return windowTitleMatchesWorkspace(windowTitle, workspace)
}
