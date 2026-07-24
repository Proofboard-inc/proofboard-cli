package state

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestStoreSaveRepairsExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not report POSIX permission bits")
	}
	homeDir := t.TempDir()
	store := NewStore(homeDir)
	directory := filepath.Dir(store.Path())
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create permissive directory: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("create permissive state file: %v", err)
	}
	if err := store.Save(context.Background(), Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat state directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("state directory mode = %v, want 0700", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

func TestWorkspaceSuppressionUsesOneWayPathKey(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "confidential-payment-platform")
	state := Default()
	state, err := AddWorkspaceSuppression(state, workspace)
	if err != nil {
		t.Fatalf("AddWorkspaceSuppression: %v", err)
	}
	if len(state.SuppressedWorkspaces) != 1 || !isSHA256(state.SuppressedWorkspaces[0]) {
		t.Fatalf("suppression state = %#v", state.SuppressedWorkspaces)
	}
	if bytes.Contains([]byte(state.SuppressedWorkspaces[0]), []byte("confidential-payment-platform")) {
		t.Fatal("suppression key retained repository name")
	}
	if !IsWorkspaceSuppressed(state, workspace) {
		t.Fatal("hashed suppression did not match its workspace")
	}
}

func TestStoreLoadMigratesLegacyWorkspacePaths(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "secret-client-repository")
	store := NewStore(homeDir)
	legacy := Default()
	legacy.SuppressedWorkspaces = []string{workspace, workspace}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	current, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(current.SuppressedWorkspaces) != 1 || !isSHA256(current.SuppressedWorkspaces[0]) {
		t.Fatalf("migrated suppression state = %#v", current.SuppressedWorkspaces)
	}
	if !IsWorkspaceSuppressed(current, workspace) {
		t.Fatal("migrated suppression no longer matches its workspace")
	}
	persisted, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read migrated state: %v", err)
	}
	if bytes.Contains(persisted, []byte("secret-client-repository")) {
		t.Fatalf("migrated state retained repository path: %s", persisted)
	}
}
