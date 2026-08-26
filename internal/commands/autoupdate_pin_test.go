package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/proofboard/proofboard/internal/version"
)

// The installer re-resolves "latest" on its own and, when neither lookup
// succeeds, falls back to a version pinned inside the script. The updater
// already knows which release it decided to install, so it must say so —
// otherwise a machine that cannot reach either lookup installs the pinned
// version instead, which can be OLDER than what is already running, silently
// and once a day.
func currentVersion() string { return version.Version }

func TestInstallerIsPinnedToTheResolvedRelease(t *testing.T) {
	// This drives scripts/install.sh, the POSIX installer. Windows updates
	// through install.ps1 instead, and running a shell stub under Git Bash
	// there only tests how Git Bash rewrites paths — it wrote its output
	// somewhere the test could not read it back, which is a fact about the
	// harness rather than about the updater.
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer is not the update path on Windows")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "install.sh")
	out := filepath.Join(dir, "seen")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$PROOFBOARD_VERSION\" > "+out+"\n"), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	cmd := exec.Command("sh", script)
	cmd.Env = append(os.Environ(), "PROOFBOARD_VERSION=v9.9.9")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run stub: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stub output: %v", err)
	}
	if string(got) != "v9.9.9" {
		t.Fatalf("installer saw PROOFBOARD_VERSION=%q, want the tag the updater resolved", got)
	}
}

// A hand-maintained pin goes stale silently: it sat at v1.10.0 while four
// releases shipped. It must always name the current version, and the release
// workflow stamps it so it cannot drift.
func TestInstallerPinnedVersionMatchesTheShippedVersion(t *testing.T) {
	for _, tc := range []struct{ path, prefix string }{
		{"../../scripts/install.sh", `PINNED_VERSION="v`},
		{"../../scripts/install.ps1", `$PinnedVersion = 'v`},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		idx := strings.Index(string(body), tc.prefix)
		if idx < 0 {
			t.Fatalf("%s: no pinned version found", tc.path)
		}
		rest := string(body)[idx+len(tc.prefix):]
		pinned := rest[:strings.IndexAny(rest, `"'`)]
		if pinned != currentVersion() {
			t.Errorf("%s pins v%s but this build is v%s — the fallback would install a different version than the one released",
				tc.path, pinned, currentVersion())
		}
	}
}
