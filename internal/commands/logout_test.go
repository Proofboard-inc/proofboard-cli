package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
)

func signedInHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
	if err := pbauth.NewCredentialStore(home).Save(context.Background(),
		model.Credentials{Token: "test-token", EmailHash: "hash"}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	dir := filepath.Join(home, ".proofboard")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	for _, f := range []string{"sync.log", "sync.log.1", "auto-update.log", "device.key", "dictionary.json"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatalf("seed %s: %v", f, err)
		}
	}
	return home
}

// Signing out must be reachable directly, not only under `auth`, and both
// spellings must do the same thing rather than one being a stub.
func TestLogoutIsReachableBothWays(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(context.Context, *bytes.Buffer) error
	}{
		{"proofboard logout", func(ctx context.Context, out *bytes.Buffer) error {
			c := newLogoutCommand(ctx, out)
			c.SetOut(out)
			return c.ExecuteContext(ctx)
		}},
		{"proofboard auth logout", func(ctx context.Context, out *bytes.Buffer) error {
			c := newAuthLogoutCommand(ctx, out)
			c.SetOut(out)
			return c.ExecuteContext(ctx)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := signedInHome(t)
			ctx := context.Background()

			var out bytes.Buffer
			if err := tc.run(ctx, &out); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(out.String(), "logged out") {
				t.Fatalf("%s printed no confirmation: %q", tc.name, out.String())
			}
			if _, err := os.Stat(filepath.Join(home, ".proofboard", "credentials.json")); !os.IsNotExist(err) {
				t.Fatalf("%s left the credentials file in place", tc.name)
			}
			// The activity log and the device key belong to the account that
			// just signed out; leaving them means "logged out" described only
			// the token.
			for _, leftover := range []string{"sync.log", "sync.log.1", "auto-update.log", "device.key"} {
				if _, err := os.Stat(filepath.Join(home, ".proofboard", leftover)); !os.IsNotExist(err) {
					t.Errorf("%s left %s behind", tc.name, leftover)
				}
			}
			// The dictionary is public reference data, identical for every
			// account, so re-downloading it after sign-in would be waste.
			if _, err := os.Stat(filepath.Join(home, ".proofboard", "dictionary.json")); os.IsNotExist(err) {
				t.Errorf("%s removed the shared dictionary, which is not account data", tc.name)
			}
			// Recorded as deliberate, so the agent does not treat it as an
			// expired session and immediately prompt to reconnect.
			st, err := statestore.NewStore(home).Load(ctx)
			if err != nil {
				t.Fatalf("load state: %v", err)
			}
			if !st.AuthLoggedOut {
				t.Fatalf("%s did not record the sign-out as deliberate", tc.name)
			}
		})
	}
}
