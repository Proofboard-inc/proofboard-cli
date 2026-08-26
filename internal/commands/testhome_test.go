package commands

import (
	"os"
	"runtime"
	"testing"
)

// setTestHome points the process at a scratch home directory for the duration
// of a test.
//
// Setting HOME alone is not enough and was the reason these tests proved
// nothing on Windows: os.UserHomeDir, which every code path here uses to find
// the Proofboard directory, reads USERPROFILE on Windows and HOME everywhere
// else. A test that set only HOME therefore ran the product against the real
// user profile on Windows — writing credentials and state outside the temp
// directory, and failing with "The system cannot find the path specified"
// when it looked for fixtures that had been written to HOME instead.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	// Credentials go to the OS secret store by default, which is per-user and
	// lives nowhere near the home directory — Windows Credential Manager, the
	// macOS login keychain. A test that redirected only HOME still wrote real
	// credentials to the real machine and read back whatever a previous test
	// had left there, which is how an end-to-end auth test came to report
	// "Already authenticated as Proofboard user" on a fresh temp home.
	//
	// The tests that exercise the keychain deliberately inject their own
	// in-memory secret store instead of calling this, so nothing that means to
	// test that path is affected.
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
}

// requireOwnerOnlyMode asserts a file is readable only by its owner.
//
// Windows has no Unix permission bits. os.Stat reports 0666 for any writable
// file there whatever its ACL says, so comparing against 0600 tests nothing and
// fails for a reason unrelated to how the file is actually protected. The
// property still matters on Windows — it is carried by an ACL rather than a
// mode — but asserting it needs a different mechanism than this, and pretending
// a mode comparison covers it would be worse than saying so.
func requireOwnerOnlyMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("%s mode = %o, want 600", path, info.Mode().Perm())
	}
}
