package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
	t.Setenv("HOME", homeDir)
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	statePath := filepath.Join(homeDir, ".config", "Code", "User", "globalStorage", "storage.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatalf("mkdir editor state: %v", err)
	}
	data := []byte(`{"windowsState":{"lastActiveWindow":{"folder":"file://` + repoDir + `"}}}`)
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatalf("write editor state: %v", err)
	}
	seen := make(map[string]bool)
	var workspaces []string
	discoverEditorStateWorkspaces(context.Background(), seen, &workspaces)
	if len(workspaces) != 1 || workspaces[0] != repoDir {
		t.Fatalf("workspaces = %#v, want %q", workspaces, repoDir)
	}
}
