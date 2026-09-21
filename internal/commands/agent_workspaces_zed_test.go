package commands

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDiscoverZedWorkspacesFindsRepositoryFromSQLiteDB(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := t.TempDir()
	setTestHome(t, homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	dbPaths := zedDBPaths()
	if len(dbPaths) == 0 {
		t.Fatal("zedDBPaths() returned nothing")
	}
	dbPath := dbPaths[0]
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("mkdir zed db dir: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	// Only the columns discoverZedWorkspacesFromDB actually reads are
	// created, so this fixture stays a minimal stand-in for Zed's real
	// schema rather than a copy of it.
	if _, err := db.Exec(`CREATE TABLE workspaces (workspace_id INTEGER PRIMARY KEY, paths TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create workspaces table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO workspaces (workspace_id, paths) VALUES (1, ?)`, repoDir); err != nil {
		db.Close()
		t.Fatalf("insert workspace row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite db: %v", err)
	}

	seen := make(map[string]bool)
	var workspaces []string
	discoverZedWorkspaces(context.Background(), seen, &workspaces)

	if len(workspaces) != 1 {
		t.Fatalf("workspaces = %#v, want exactly the repo", workspaces)
	}
	got, err := filepath.EvalSymlinks(workspaces[0])
	if err != nil {
		t.Fatalf("resolve workspace path: %v", err)
	}
	want, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("resolve repo path: %v", err)
	}
	if got != want {
		t.Fatalf("workspace = %q, want %q", got, want)
	}
}

func TestDiscoverZedWorkspacesSkipsMissingDB(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)

	seen := make(map[string]bool)
	var workspaces []string
	discoverZedWorkspaces(context.Background(), seen, &workspaces)

	if len(workspaces) != 0 {
		t.Fatalf("workspaces = %#v, want none when no db exists", workspaces)
	}
}
