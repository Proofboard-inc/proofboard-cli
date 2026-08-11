//go:build linux

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// openTerminalRunning opens a terminal emulator (first one found on PATH)
// and runs execPath with args in workspace's directory, so the user can
// watch it happen. Falls back to a silent background run only if no
// terminal emulator can be found at all (e.g. a headless/minimal desktop).
func openTerminalRunning(ctx context.Context, workspace, execPath string, args []string) error {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, strconv.Quote(execPath))
	for _, a := range args {
		quoted = append(quoted, strconv.Quote(a))
	}
	shellCmd := fmt.Sprintf("cd %s && %s; exec $SHELL", strconv.Quote(workspace), strings.Join(quoted, " "))

	for _, candidate := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xterm"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		var c *exec.Cmd
		if candidate == "gnome-terminal" {
			c = exec.CommandContext(ctx, path, "--", "bash", "-c", shellCmd)
		} else {
			c = exec.CommandContext(ctx, path, "-e", "bash -c "+strconv.Quote(shellCmd))
		}
		if err := c.Start(); err == nil {
			return nil
		}
	}
	cmd := exec.CommandContext(ctx, execPath, args...)
	cmd.Dir = workspace
	return cmd.Start()
}
