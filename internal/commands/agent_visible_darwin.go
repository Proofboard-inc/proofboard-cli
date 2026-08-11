//go:build darwin

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// openTerminalRunning opens Terminal.app and runs execPath with args in
// workspace's directory, so the user can watch it happen.
func openTerminalRunning(ctx context.Context, workspace, execPath string, args []string) error {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, strconv.Quote(execPath))
	for _, a := range args {
		parts = append(parts, strconv.Quote(a))
	}
	shellCmd := fmt.Sprintf("cd %s && %s", strconv.Quote(workspace), strings.Join(parts, " "))
	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script %s
end tell`, strconv.Quote(shellCmd))
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}
