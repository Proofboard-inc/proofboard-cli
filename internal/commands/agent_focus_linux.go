//go:build linux

package commands

import (
	"context"
	"os/exec"
	"strings"
)

// isWorkspaceFocused reports whether the given workspace's IDE window is the
// one currently active. Best-effort only: this relies on xdotool, which
// requires X11; there is no equivalent, reliable, tooling-independent way to
// query the focused window under Wayland (out of scope here: full Wayland
// compositor protocol integration is a separate, much larger effort).
// workspaceCount is the number of IDE workspaces the agent is currently
// tracking this tick (see discoverIDEWorkspaces). When the check can't be
// performed at all (xdotool missing, e.g. under Wayland, or it errors), this
// fails open (fires) only when this is the sole tracked workspace: with
// more than one tracked workspace there's no way to tell which one is
// actually focused, so it suppresses instead of guessing.
func isWorkspaceFocused(ctx context.Context, workspace string, ideNames []string, workspaceCount int) bool {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return workspaceCount <= 1
	}
	nameOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow", "getwindowname").Output()
	if err != nil {
		return workspaceCount <= 1
	}
	windowTitle := strings.ToLower(strings.TrimSpace(string(nameOut)))

	frontApp := ""
	if pidOut, pidErr := exec.CommandContext(ctx, "xdotool", "getactivewindow", "getwindowpid").Output(); pidErr == nil {
		pid := strings.TrimSpace(string(pidOut))
		if commOut, commErr := exec.CommandContext(ctx, "ps", "-p", pid, "-o", "comm=").Output(); commErr == nil {
			frontApp = strings.TrimSpace(string(commOut))
		}
	}
	if frontApp != "" && !matchesFrontmostIDEName(frontApp, ideNames) {
		return false
	}
	return windowTitleMatchesWorkspace(windowTitle, workspace)
}
