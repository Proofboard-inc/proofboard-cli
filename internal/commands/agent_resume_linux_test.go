//go:build linux

package commands

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// On Linux without a systemd user session and without a desktop login — a
// server, a container, WSL, a Codespace — the Career Agent is registered only
// through a desktop autostart entry that nothing ever runs. After a reboot the
// agent stayed down for good: `proofboard status` said "Stopped" while every
// shell kept firing the detection hook. The shell hook is the one thing that
// still runs there, so it must bring a registered agent back.
func TestShellHookResumesRegisteredAgentAfterReboot(t *testing.T) {
	homeDir := t.TempDir()
	writeAutostartRegistration(t, homeDir)
	// The pid file survives a reboot, naming a process that no longer exists.
	writeAgentPIDFile(t, homeDir, 1<<30)

	starts := 0
	resumeRegisteredAgent(homeDir, func() error { starts++; return nil })
	if starts != 1 {
		t.Fatalf("registered agent that is not running was started %d times, want 1", starts)
	}
}

func TestShellHookLeavesUnregisteredAgentAlone(t *testing.T) {
	homeDir := t.TempDir()

	starts := 0
	resumeRegisteredAgent(homeDir, func() error { starts++; return nil })
	if starts != 0 {
		t.Fatalf("agent was started %d times without ever being installed", starts)
	}
}

func TestShellHookDoesNotStartASecondAgent(t *testing.T) {
	homeDir := t.TempDir()
	writeAutostartRegistration(t, homeDir)
	writeAgentPIDFile(t, homeDir, os.Getpid())

	starts := 0
	resumeRegisteredAgent(homeDir, func() error { starts++; return nil })
	if starts != 0 {
		t.Fatalf("a running agent was started again %d times", starts)
	}
}

// A developer who ran `proofboard agent stop` meant it. Opening a terminal
// must not quietly undo that.
func TestShellHookRespectsADeliberateStop(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	writeAutostartRegistration(t, homeDir)

	if err := stopAgentByUser(io.Discard); err != nil {
		t.Fatalf("stopAgentByUser: %v", err)
	}

	starts := 0
	resumeRegisteredAgent(homeDir, func() error { starts++; return nil })
	if starts != 0 {
		t.Fatalf("agent the developer stopped was restarted %d times by a new shell", starts)
	}

	if err := clearAgentStopped(homeDir); err != nil {
		t.Fatalf("clearAgentStopped: %v", err)
	}
	resumeRegisteredAgent(homeDir, func() error { starts++; return nil })
	if starts != 1 {
		t.Fatalf("after starting it again, a new shell resumed it %d times, want 1", starts)
	}
}

func writeAutostartRegistration(t *testing.T, homeDir string) {
	t.Helper()
	dir := filepath.Join(homeDir, ".config", "autostart")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir autostart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proofboard-career-agent.desktop"), []byte("[Desktop Entry]\n"), 0o600); err != nil {
		t.Fatalf("write autostart entry: %v", err)
	}
}

func writeAgentPIDFile(t *testing.T, homeDir string, pid int) {
	t.Helper()
	path := agentPIDPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
}
