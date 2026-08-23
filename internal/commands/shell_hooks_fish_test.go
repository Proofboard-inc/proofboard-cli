package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fish hook used to be written as two lines: a backgrounded detect and a
// `disown $last_pid` that acted on it. Migrating it must replace BOTH lines.
// Replacing only the first left a bare `disown $last_pid` with no job to act
// on, which errored on every new fish shell.
func TestFishLegacyMigrationLeavesNoOrphanedDisown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.fish")
	if err := os.WriteFile(path, []byte("# Proofboard Workspace Detection\n"+legacyFishBackgroundedDetectionLine+"\n"), 0o644); err != nil {
		t.Fatalf("seed config.fish: %v", err)
	}

	if _, _, err := ensureLineInFile(path, fishShellDetectionLine); err != nil {
		t.Fatalf("ensureLineInFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(got), "disown") {
		t.Fatalf("orphaned disown survived migration:\n%s", got)
	}
	if !strings.Contains(string(got), fishShellDetectionLine) {
		t.Fatalf("current fish hook missing after migration:\n%s", got)
	}
}

// The ordering that makes the above work must hold by construction, not by
// how the slice literal happens to be written.
func TestLegacyDetectionLinesAreLongestFirst(t *testing.T) {
	for i := 1; i < len(legacyDetectionLines); i++ {
		if len(legacyDetectionLines[i-1]) < len(legacyDetectionLines[i]) {
			t.Fatalf("legacyDetectionLines not longest-first at %d: a shorter legacy value that is a substring of a longer one would match first and migrate only part of it", i)
		}
	}
}
