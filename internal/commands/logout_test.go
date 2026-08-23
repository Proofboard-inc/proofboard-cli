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
	t.Setenv("HOME", home)
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
	if err := pbauth.NewCredentialStore(home).Save(context.Background(),
		model.Credentials{Token: "test-token", EmailHash: "hash"}); err != nil {
		t.Fatalf("seed credentials: %v", err)
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
