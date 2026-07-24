//go:build darwin

package commands

import (
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const agentLaunchdLabel = "io.proofboard.career-agent"

func installAgentService(executable string, out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	path := filepath.Join(dir, agentLaunchdLabel+".plist")
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>agent</string><string>run</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
`, agentLaunchdLabel, html.EscapeString(executable))
	if err := os.WriteFile(path, []byte(plist), 0o600); err != nil {
		return fmt.Errorf("write LaunchAgent: %w", err)
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+agentLaunchdLabel).Run()
	if err := exec.Command("launchctl", "bootstrap", domain, path).Run(); err != nil {
		return fmt.Errorf("start LaunchAgent: %w", err)
	}
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent registered to start when you sign in.")
	return nil
}

func uninstallAgentService(out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+agentLaunchdLabel).Run()
	path := filepath.Join(homeDir, "Library", "LaunchAgents", agentLaunchdLabel+".plist")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent: %w", err)
	}
	_ = stopAgent(io.Discard)
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent background service removed.")
	return nil
}
