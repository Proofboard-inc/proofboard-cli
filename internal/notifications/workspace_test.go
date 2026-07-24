package notifications

import (
	"context"
	"os"
	"path/filepath"
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
	want, _ := filepath.Abs(workspace)
	if len(current.SuppressedWorkspaces) != 1 || current.SuppressedWorkspaces[0] != want {
		t.Fatalf("suppressed workspaces = %#v, want %q", current.SuppressedWorkspaces, want)
	}
	info, err := os.Stat(state.NewStore(homeDir).Path())
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode = %v", info.Mode().Perm())
	}
}

func TestWorkspaceActionLabelsExposeThreeChoices(t *testing.T) {
	_, _, primary, secondary, tertiary := workspaceActionLabels("link")
	if primary != "Sync Project" || secondary != "Not Now" || tertiary != "Never Ask Again" {
		t.Fatalf("unexpected choices: %q, %q, %q", primary, secondary, tertiary)
	}
}

func TestMilestoneActionsRouteToDashboard(t *testing.T) {
	_, _, primary, secondary, tertiary := workspaceActionLabels("milestone")
	primaryKey, secondaryKey, tertiaryKey := workspaceActionKeys("milestone")
	if primary != "Review" || secondary != "Publish" || tertiary != "Ignore" {
		t.Fatalf("unexpected milestone labels: %q, %q, %q", primary, secondary, tertiary)
	}
	if primaryKey != "review" || secondaryKey != "publish" || tertiaryKey != "ignore" {
		t.Fatalf("unexpected milestone keys: %q, %q, %q", primaryKey, secondaryKey, tertiaryKey)
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
