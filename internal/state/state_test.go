package state

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func TestStoreSaveCreates0600StateFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not report POSIX permission bits")
	}
	store := NewStore(t.TempDir())
	if err := store.Save(context.Background(), model.State{LinkedRepos: map[string]model.LinkedRepoState{}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 state file, got %v", info.Mode().Perm())
	}
}
