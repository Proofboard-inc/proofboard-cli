//go:build linux

package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverIDEWorkspacesFromRunningProcess(t *testing.T) {
	repoDir := t.TempDir()
	gitInit := exec.Command("git", "init", "--initial-branch=main")
	gitInit.Dir = repoDir
	if output, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	sleepBinary, err := os.ReadFile("/bin/sleep")
	if err != nil {
		t.Fatalf("read sleep executable: %v", err)
	}
	fakeIDE := filepath.Join(t.TempDir(), "pbide")
	if err := os.WriteFile(fakeIDE, sleepBinary, 0o700); err != nil {
		t.Fatalf("write fake IDE: %v", err)
	}

	process := exec.Command(fakeIDE, "10")
	process.Dir = repoDir
	if err := process.Start(); err != nil {
		t.Fatalf("start fake IDE: %v", err)
	}
	t.Cleanup(func() {
		_ = process.Process.Kill()
		_ = process.Wait()
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		workspaces, discoverErr := discoverIDEWorkspaces(context.Background(), []string{"pbide"})
		if discoverErr != nil {
			t.Fatalf("discover IDE workspaces: %v", discoverErr)
		}
		for _, workspace := range workspaces {
			if workspace == repoDir {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("running IDE workspace %q not found in %#v", repoDir, workspaces)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestDesktopNotificationsRequireLinuxDesktopSession(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if desktopNotificationsAvailable() {
		t.Fatal("desktop notifications reported available without a Linux desktop session")
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/proofboard-test-session-bus")
	if !desktopNotificationsAvailable() {
		t.Fatal("desktop notifications reported unavailable with a session bus")
	}
}
