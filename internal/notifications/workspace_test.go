package notifications

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/proofboard/proofboard/internal/state"
)

func TestSuppressWorkspacePersistsNeverAskAgain(t *testing.T) {
	homeDir := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := ActivateWorkspaceAction(context.Background(), "suppress", workspace); err != nil {
		t.Fatalf("ActivateWorkspaceAction() error: %v", err)
	}
	current, err := state.NewStore(homeDir).Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	want, err := state.WorkspaceSuppressionKey(workspace)
	if err != nil {
		t.Fatalf("suppression key: %v", err)
	}
	if len(current.SuppressedWorkspaces) != 1 || current.SuppressedWorkspaces[0] != want {
		t.Fatalf("suppressed workspaces = %#v, want %q", current.SuppressedWorkspaces, want)
	}
	persisted, err := os.ReadFile(state.NewStore(homeDir).Path())
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	if bytes.Contains(persisted, []byte(workspace)) {
		t.Fatalf("state file retained proprietary workspace path: %s", persisted)
	}
	info, err := os.Stat(state.NewStore(homeDir).Path())
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode = %v", info.Mode().Perm())
	}
}

// reconnect (session expiry) is the only case reserved for an OS-level
// popup now — project-detected, sync-needed, and milestone-ready all moved
// to plain terminal output and must not carry the old interactive labels
// (Sync Project/Review/Publish/etc.) that implied a clickable popup exists
// for them.
func TestWorkspaceActionLabelsOnlyReconnectIsARealPopup(t *testing.T) {
	title, body, primary, secondary, tertiary := workspaceActionLabels("reconnect")
	if title != "Your Proofboard session has expired" {
		t.Fatalf("unexpected reconnect prompt title: %q", title)
	}
	if body == "" || primary != "Reconnect" || secondary != "" || tertiary != "" {
		t.Fatalf("unexpected reconnect choices: %q / %q, %q, %q", body, primary, secondary, tertiary)
	}

	for _, kind := range []string{"link", "sync", "milestone"} {
		title, _, primary, secondary, tertiary := workspaceActionLabels(kind)
		if title != "Proofboard Career Agent" || primary != "Open" || secondary != "Not Now" || tertiary != "Never Ask Again" {
			t.Fatalf("%q should fall through to the generic default, got: %q / %q, %q, %q", kind, title, primary, secondary, tertiary)
		}
	}
}

// FIX: ActivateWorkspaceAction's "Review"/"Publish" (with no known bundle)
// fallback used to open a hardcoded "https://proofboard.io/dashboard" — the
// release/download domain, not the deployed frontend app — so clicking those
// buttons sent the user to the wrong site. appBaseURL must resolve the CLI's
// actual configured app URL instead.
func TestAppBaseURLRespectsConfigOverride(t *testing.T) {
	t.Setenv("PROOFBOARD_APP_BASE_URL", "https://custom.example.com")
	if got := appBaseURL(context.Background()); got != "https://custom.example.com" {
		t.Fatalf("appBaseURL() = %q, want the configured override", got)
	}
}

func TestAppBaseURLFallsBackToDefaultFrontend(t *testing.T) {
	t.Setenv("PROOFBOARD_APP_BASE_URL", "")
	got := appBaseURL(context.Background())
	if got != "https://proofboard-frontend.vercel.app" {
		t.Fatalf("appBaseURL() = %q, want the real default frontend app, not proofboard.io", got)
	}
}

func TestWorkspaceTargetActionURIRoundTrip(t *testing.T) {
	raw := WorkspaceTargetActionURI("publish", "", "bundle-123")
	kind, workspace, target, err := ParseWorkspaceTargetActionURI(raw)
	if err != nil {
		t.Fatalf("ParseWorkspaceTargetActionURI() error: %v", err)
	}
	if kind != "publish" || workspace != "" || target != "bundle-123" {
		t.Fatalf("unexpected action: kind=%q workspace=%q target=%q", kind, workspace, target)
	}
}
