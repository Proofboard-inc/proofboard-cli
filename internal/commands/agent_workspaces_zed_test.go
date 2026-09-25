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

// Zed keeps every root of a multi-root workspace in one row, newline
// separated. A path containing a comma is a single root, not two.
func TestDiscoverZedWorkspacesReadsEveryRootOfAMultiRootWorkspace(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)

	parent := t.TempDir()
	var roots []string
	for _, name := range []string{"frontend", "api, v2"} {
		root := filepath.Join(parent, name)
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", root, err)
		}
		cmd := exec.Command("git", "init")
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v: %s", root, err, output)
		}
		roots = append(roots, root)
	}

	dbPath := zedDBPaths()[0]
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("mkdir zed db dir: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE workspaces (workspace_id INTEGER PRIMARY KEY, paths TEXT, paths_order TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create workspaces table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO workspaces (workspace_id, paths, paths_order) VALUES (1, ?, '1,0')`, roots[0]+"\n"+roots[1]); err != nil {
		db.Close()
		t.Fatalf("insert workspace row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite db: %v", err)
	}

	seen := make(map[string]bool)
	var workspaces []string
	discoverZedWorkspaces(context.Background(), seen, &workspaces)

	found := make(map[string]bool)
	for _, workspace := range workspaces {
		resolved, err := filepath.EvalSymlinks(workspace)
		if err != nil {
			t.Fatalf("resolve workspace path: %v", err)
		}
		found[resolved] = true
	}
	for _, root := range roots {
		want, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("resolve root: %v", err)
		}
		if !found[want] {
			t.Fatalf("root %q not discovered; workspaces = %#v", want, workspaces)
		}
	}
	if len(workspaces) != len(roots) {
		t.Fatalf("workspaces = %#v, want exactly %d roots", workspaces, len(roots))
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
