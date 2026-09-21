package commands

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// zedDBPaths returns Zed's per-OS workspace database locations. Zed keeps a
// separate database per release channel ("0-stable", "0-dev", …); both of
// the ones a normal install would use are checked.
func zedDBPaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(homeDir, "Library", "Application Support", "Zed")
	case "windows":
		base = filepath.Join(os.Getenv("APPDATA"), "Zed")
	default:
		base = filepath.Join(homeDir, ".local", "share", "zed")
	}
	return []string{
		filepath.Join(base, "db", "0-stable", "db.sqlite"),
		filepath.Join(base, "db", "0-dev", "db.sqlite"),
	}
}

// discoverZedWorkspaces reads Zed's recent-workspace list. Unlike every other
// supported editor, Zed does not keep this in a JSON file: it stores it in a
// SQLite database (workspaces.paths), so it needs its own driver rather than
// discoverEditorStateWorkspaces's JSON walk.
func discoverZedWorkspaces(ctx context.Context, seen map[string]bool, workspaces *[]string) {
	for _, path := range zedDBPaths() {
		discoverZedWorkspacesFromDB(ctx, path, seen, workspaces)
	}
}

func discoverZedWorkspacesFromDB(ctx context.Context, path string, seen map[string]bool, workspaces *[]string) {
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return
	}
	// Opened read-only and immutable: this file may be held open by a running
	// Zed instance, and the Career Agent has no business writing to it.
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1")
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `SELECT paths FROM workspaces WHERE paths IS NOT NULL AND paths != ''`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var paths string
		if rows.Scan(&paths) != nil {
			continue
		}
		// A multi-root Zed workspace stores its roots comma-joined in this
		// column (confirmed against this machine's own single-root entries;
		// no multi-root sample was available to verify the separator
		// directly, so this is the best-effort reading of the schema).
		for _, candidate := range strings.Split(paths, ",") {
			addWorkspaceCandidate(ctx, candidate, seen, workspaces)
		}
	}
}
