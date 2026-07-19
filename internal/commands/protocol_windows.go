//go:build windows

package commands

import (
	"fmt"
	"os/exec"
)

func registerProtocolHandler(execPath string) error {
	cmd := exec.Command("reg", "add", `HKCU\Software\Classes\proofboard`, "/ve", "/d", "URL:Proofboard Protocol", "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set protocol name: %w (%s)", err, string(out))
	}
	cmd = exec.Command("reg", "add", `HKCU\Software\Classes\proofboard`, "/v", "URL Protocol", "/d", "", "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mark protocol: %w (%s)", err, string(out))
	}
	commandValue := fmt.Sprintf(`"%s" notify-activate "%%1"`, execPath)
	cmd = exec.Command("reg", "add", `HKCU\Software\Classes\proofboard\shell\open\command`, "/ve", "/d", commandValue, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set protocol command: %w (%s)", err, string(out))
	}
	return nil
}

func unregisterProtocolHandler() error {
	cmd := exec.Command("reg", "delete", `HKCU\Software\Classes\proofboard`, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove protocol handler: %w (%s)", err, string(out))
	}
	return nil
}
