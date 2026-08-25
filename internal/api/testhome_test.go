package api

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
}
