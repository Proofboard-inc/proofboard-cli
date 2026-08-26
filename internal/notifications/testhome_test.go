package notifications

import "testing"

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
