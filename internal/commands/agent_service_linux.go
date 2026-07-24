//go:build linux

package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func installAgentService(executable string, out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	serviceDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0o700); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}
	servicePath := filepath.Join(serviceDir, "proofboard-career-agent.service")
	unit := fmt.Sprintf("[Unit]\nDescription=Proofboard Career Agent\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart=%q agent run\nRestart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n", executable)
	if err := os.WriteFile(servicePath, []byte(unit), 0o600); err != nil {
		return fmt.Errorf("write systemd user service: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err == nil {
		if err := exec.Command("systemctl", "--user", "enable", "--now", "proofboard-career-agent.service").Run(); err == nil {
			_, _ = fmt.Fprintln(out, "Proofboard Career Agent registered to start when you sign in.")
			return nil
		}
	}

	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	if err := os.MkdirAll(autostartDir, 0o700); err != nil {
		return fmt.Errorf("create desktop autostart directory: %w", err)
	}
	desktopEntry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Proofboard Career Agent\nExec=%q agent run\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", executable)
	if err := os.WriteFile(filepath.Join(autostartDir, "proofboard-career-agent.desktop"), []byte(desktopEntry), 0o600); err != nil {
		return fmt.Errorf("write desktop autostart entry: %w", err)
	}
	cmd := exec.Command(executable, "agent", "run")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := startDetachedCommand(cmd); err != nil {
		return fmt.Errorf("start Career Agent: %w", err)
	}
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent registered with desktop autostart.")
	return nil
}

func uninstallAgentService(out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", "proofboard-career-agent.service").Run()
	for _, path := range []string{
		filepath.Join(homeDir, ".config", "systemd", "user", "proofboard-career-agent.service"),
		filepath.Join(homeDir, ".config", "autostart", "proofboard-career-agent.desktop"),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove Career Agent registration: %w", err)
		}
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	_ = stopAgent(io.Discard)
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent background service removed.")
	return nil
}
