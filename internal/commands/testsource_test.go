package commands

import (
	"os"
	"testing"
)

// readSource lets a test assert on wiring that has no runtime seam — here,
// that the reconnect passes --force. Asserting on behaviour is preferred, but
// the sign-in flow opens a browser and blocks on a device code, so there is
// no way to exercise it in a unit test without a fake, which this project
// does not allow.
func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
