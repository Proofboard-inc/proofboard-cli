package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSplitCommandLinePreservesWorkspaceWithSpaces(t *testing.T) {
	fields := splitCommandLine(`123 code /usr/bin/code "/work/Payment Platform" --reuse-window`)
	want := "/work/Payment Platform"
	found := false
	for _, field := range fields {
		if field == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("splitCommandLine() = %#v, missing %q", fields, want)
	}
}

func TestSplitCommandLinePreservesWindowsPathSeparators(t *testing.T) {
	fields := splitCommandLine(`code.exe "C:\Users\Ada Lovelace\project"`)
	if len(fields) != 2 || fields[1] != `C:\Users\Ada Lovelace\project` {
		t.Fatalf("split fields = %#v", fields)
	}
}

func TestDiscoverEditorStateWorkspacesFindsLastActiveRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := t.TempDir()
	setTestHome(t, homeDir)
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	// Use whatever path editorStateFiles() actually looks at for this OS
	// (e.g. ~/Library/Application Support/Code/... on macOS) rather than a
	// hardcoded Linux-only ~/.config path — otherwise this test writes its
	// fixture somewhere discoverEditorStateWorkspaces never looks on darwin.
	statePaths := editorStateFiles()
	if len(statePaths) == 0 {
		t.Fatal("editorStateFiles() returned no candidate paths")
	}
	statePath := statePaths[0]
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatalf("mkdir editor state: %v", err)
	}
	// Written the way the editor actually writes it, which differs by platform:
	// a Windows folder URI carries the drive inside the path and uses forward
	// slashes ("file:///C:/Users/..."), so pasting a native path after
	// "file://" produced a URI no editor would ever emit and a test that could
	// only ever fail there.
	folderURI := "file://" + filepath.ToSlash(repoDir)
	if runtime.GOOS == "windows" {
		folderURI = "file:///" + filepath.ToSlash(repoDir)
	}
	data := []byte(`{"windowsState":{"lastActiveWindow":{"folder":"` + folderURI + `"}}}`)
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatalf("write editor state: %v", err)
	}
	// git resolves symlinks in its reported toplevel path (relevant on macOS,
	// where a temp dir is typically under /var/folders, itself a symlink to
	// /private/var/folders) — resolve the expected path the same way so the
	// comparison isn't just an artifact of an unresolved vs. resolved path.
	wantRepoDir := repoDir
	if resolved, err := filepath.EvalSymlinks(repoDir); err == nil {
		wantRepoDir = resolved
	}
	seen := make(map[string]bool)
	var workspaces []string
	discoverEditorStateWorkspaces(context.Background(), seen, &workspaces)
	if len(workspaces) != 1 || workspaces[0] != wantRepoDir {
		t.Fatalf("workspaces = %#v, want %q", workspaces, wantRepoDir)
	}
}
