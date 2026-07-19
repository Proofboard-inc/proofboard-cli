//go:build windows

package commands

import (
	"os/exec"
	"syscall"
)

func startDetachedCommand(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
