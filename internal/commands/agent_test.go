package commands

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAgentPIDLifecycle(t *testing.T) {
	homeDir := t.TempDir()
	if err := claimAgentPID(homeDir); err != nil {
		t.Fatalf("claimAgentPID() error: %v", err)
	}
	if running, pid := agentRunning(homeDir); !running || pid != os.Getpid() {
		t.Fatalf("agentRunning() = %v, %d", running, pid)
	}
	info, err := os.Stat(agentPIDPath(homeDir))
	if err != nil {
		t.Fatalf("stat pid file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pid file mode = %v", info.Mode().Perm())
	}
	releaseAgentPID(homeDir, os.Getpid())
	if _, err := os.Stat(agentPIDPath(homeDir)); !os.IsNotExist(err) {
		t.Fatalf("expected pid file removal, err=%v", err)
	}
}

func TestAgentIgnoresStalePID(t *testing.T) {
	homeDir := t.TempDir()
	path := agentPIDPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(1<<30)), 0o600); err != nil {
		t.Fatalf("write stale pid: %v", err)
	}
	if running, _ := agentRunning(homeDir); running {
		t.Fatal("expected stale pid to report stopped")
	}
}

func TestMatchesIDEProcess(t *testing.T) {
	if !matchesIDEProcess("/usr/bin/code", []string{"code", "cursor"}) {
		t.Fatal("expected code process to match")
	}
	if matchesIDEProcess("/usr/bin/git", []string{"code", "cursor"}) {
		t.Fatal("did not expect git process to match")
	}
}

func TestPruneInactiveWorkspaceSessionsResetsNotNowAfterWorkspaceCloses(t *testing.T) {
	seenUnlinked := map[string]bool{
		"/workspace/open":   true,
		"/workspace/closed": true,
	}
	lastSyncLaunch := map[string]time.Time{
		"/workspace/open":   time.Now(),
		"/workspace/closed": time.Now(),
	}
	active := map[string]bool{"/workspace/open": true}

	pruneInactiveWorkspaceSessions(seenUnlinked, lastSyncLaunch, active)

	if !seenUnlinked["/workspace/open"] {
		t.Fatal("active workspace prompt session was removed")
	}
	if seenUnlinked["/workspace/closed"] {
		t.Fatal("closed workspace remained dismissed after its IDE session ended")
	}
	if _, exists := lastSyncLaunch["/workspace/closed"]; exists {
		t.Fatal("closed workspace retained its sync throttle")
	}
}
