//go:build windows

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// openTerminalRunning opens a terminal window (Windows Terminal if
// available, otherwise classic cmd.exe) and runs execPath with args in
// workspace's directory, so the user can watch it happen.
func openTerminalRunning(ctx context.Context, workspace, execPath string, args []string) error {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, fmt.Sprintf("%q", execPath))
	for _, a := range args {
		quoted = append(quoted, fmt.Sprintf("%q", a))
	}
	cmdLine := strings.Join(quoted, " ")
	if _, err := exec.LookPath("wt.exe"); err == nil {
		c := exec.CommandContext(ctx, "wt.exe", "-d", workspace, "cmd", "/K", cmdLine)
		return c.Start()
	}
	c := exec.CommandContext(ctx, "cmd", "/C", "start", "Proofboard", "cmd", "/K",
		fmt.Sprintf("cd /d %q && %s", workspace, cmdLine))
	return c.Start()
}
