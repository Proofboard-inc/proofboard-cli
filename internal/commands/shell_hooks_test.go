package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLineInFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")

	updated, err := ensureLineInFile(path, shellDetectionLine)
	if err != nil {
		t.Fatalf("ensureLineInFile first write failed: %v", err)
	}
	if !updated {
		t.Fatalf("expected first write to report updated")
	}

	updated, err = ensureLineInFile(path, shellDetectionLine)
	if err != nil {
		t.Fatalf("ensureLineInFile second write failed: %v", err)
	}
	if updated {
		t.Fatalf("expected second write to be a no-op")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}

	content := string(data)
	if strings.Count(content, shellDetectionLine) != 1 {
		t.Fatalf("expected exactly one detection hook, got content: %q", content)
	}
}

func TestEnsureLineInFile_MigratesTrackedBackgroundJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	content := "# Proofboard Workspace Detection\n" + legacyShellDetectionLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("write legacy hook: %v", err)
	}

	updated, err := ensureLineInFile(path, shellDetectionLine)
	if err != nil || !updated {
		t.Fatalf("migration = %v, %v", updated, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated hook: %v", err)
	}
	if strings.Contains(string(data), "\n"+legacyShellDetectionLine+"\n") {
		t.Fatalf("legacy hook remains: %q", data)
	}
	if strings.Count(string(data), shellDetectionLine) != 1 {
		t.Fatalf("migrated hook count = %d", strings.Count(string(data), shellDetectionLine))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat migrated hook: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

// Regression test for the "detect never actually prints anything" bug: an
// install whose rc file still has the old backgrounded-and-fully-silenced
// line (`(proofboard detect >/dev/null 2>&1 &)`, both stdout AND stderr
// discarded) must get migrated onto the new synchronous line, not left with
// a hook that can never surface a prompt.
func TestEnsureLineInFile_MigratesLegacyBackgroundedDetectLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := "# Proofboard Workspace Detection\n" + legacyBackgroundedShellDetectionLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("write legacy hook: %v", err)
	}

	updated, err := ensureLineInFile(path, shellDetectionLine)
	if err != nil || !updated {
		t.Fatalf("migration = %v, %v", updated, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated hook: %v", err)
	}
	if strings.Contains(string(data), legacyBackgroundedShellDetectionLine) {
		t.Fatalf("legacy backgrounded hook remains: %q", data)
	}
	if !strings.Contains(string(data), shellDetectionLine) {
		t.Fatalf("expected migrated hook to contain %q, got: %q", shellDetectionLine, data)
	}
	// The old line discarded stdout too (">/dev/null 2>&1", plus backgrounded
	// with "&") — the new one must only suppress stderr and run synchronously.
	if strings.Contains(shellDetectionLine, "1>/dev/null") || strings.Contains(shellDetectionLine, "2>&1") || strings.HasSuffix(strings.TrimSpace(shellDetectionLine), "&") {
		t.Fatalf("new detect line must not discard stdout or run backgrounded: %q", shellDetectionLine)
	}
}

func TestEnsureShellDetectionHooksWritesBothDetectAndNoticeLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")

	for i := 0; i < 2; i++ {
		for _, line := range []string{shellDetectionLine, noticeLine} {
			if _, err := ensureLineInFile(path, line); err != nil {
				t.Fatalf("ensureLineInFile(%q) run %d: %v", line, i, err)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	content := string(data)
	if strings.Count(content, shellDetectionLine) != 1 {
		t.Fatalf("expected exactly one detect hook, got: %q", content)
	}
	if strings.Count(content, noticeLine) != 1 {
		t.Fatalf("expected exactly one notice hook, got: %q", content)
	}
	// The notice line must run synchronously (not backgrounded/silenced the
	// way detect is) so its output is actually visible on shell startup.
	if strings.Contains(noticeLine, ">/dev/null 2>&1 &") {
		t.Fatalf("notice line must not be backgrounded/fully silenced: %q", noticeLine)
	}
}

func TestIsInternalCommand(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"notify":          {args: []string{"notify"}, want: true},
		"notify-activate": {args: []string{"notify-activate"}, want: true},
		"hook-maintain":   {args: []string{"hook-maintain"}, want: true},
		"sync":            {args: []string{"sync"}, want: false},
		"empty":           {args: nil, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isInternalCommand(tc.args); got != tc.want {
				t.Fatalf("isInternalCommand(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
