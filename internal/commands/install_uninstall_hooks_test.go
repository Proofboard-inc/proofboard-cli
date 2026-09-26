package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveAllHeaderBlocksRemovesEveryOccurrence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := strings.Join([]string{
		"export EDITOR=vim",
		"",
		workspaceDetectionHeader,
		`[[ -n "$PROOFBOARD_SKIP" ]] || proofboard detect 2>/dev/null`,
		"",
		workspaceDetectionHeader,
		`proofboard notices 2>/dev/null`,
		"",
		"alias ll='ls -la'",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}

	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	result := string(got)
	if strings.Contains(result, workspaceDetectionHeader) {
		t.Errorf("header still present: %q", result)
	}
	if strings.Contains(result, "proofboard detect") || strings.Contains(result, "proofboard notices") {
		t.Errorf("hook lines still present: %q", result)
	}
	if !strings.Contains(result, "export EDITOR=vim") || !strings.Contains(result, "alias ll='ls -la'") {
		t.Errorf("unrelated lines were dropped: %q", result)
	}
}

func TestRemoveAllHeaderBlocksNoopWhenHeaderAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := "export EDITOR=vim\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}
	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	if string(got) != content {
		t.Errorf("file changed when header was absent: %q", string(got))
	}
}

func TestRemoveAllHeaderBlocksNoopWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks on missing file: %v", err)
	}
}

// FIX: performUninstall previously only stripped the PATH export header
// (proofboardPathHeader) via removeDirectoryFromPath, leaving the
// workspace-detection and autocompletion header blocks behind in every rc
// file they were written to. removeShellHookBlocks must strip all three.
func TestRemoveShellHookBlocksStripsAllThreeHeadersFromEveryRcFile(t *testing.T) {
	homeDir := t.TempDir()
	env := installEnvironment{GOOS: "linux", HomeDir: homeDir}

	zshrc := filepath.Join(homeDir, ".zshrc")
	zshrcContent := strings.Join([]string{
		proofboardPathHeader,
		`export PATH="/home/engineer/.local/bin:$PATH"`,
		"",
		workspaceDetectionHeader,
		"proofboard detect 2>/dev/null",
		"",
		autocompletionHeader,
		"source <(proofboard completion zsh)",
		"",
		"# user's own config below",
		"alias gs='git status'",
	}, "\n")
	if err := os.WriteFile(zshrc, []byte(zshrcContent), 0o644); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	zprofile := filepath.Join(homeDir, ".zprofile")
	zprofileContent := strings.Join([]string{
		proofboardPathHeader,
		`export PATH="/home/engineer/.local/bin:$PATH"`,
	}, "\n")
	if err := os.WriteFile(zprofile, []byte(zprofileContent), 0o644); err != nil {
		t.Fatalf("write .zprofile: %v", err)
	}

	removeShellHookBlocks(env)

	zshrcGot, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	for _, header := range []string{proofboardPathHeader, workspaceDetectionHeader, autocompletionHeader} {
		if strings.Contains(string(zshrcGot), header) {
			t.Errorf(".zshrc still contains header %q: %q", header, zshrcGot)
		}
	}
	if !strings.Contains(string(zshrcGot), "alias gs='git status'") {
		t.Errorf(".zshrc lost unrelated content: %q", zshrcGot)
	}

	zprofileGot, err := os.ReadFile(zprofile)
	if err != nil {
		t.Fatalf("read .zprofile: %v", err)
	}
	if strings.Contains(string(zprofileGot), proofboardPathHeader) {
		t.Errorf(".zprofile still contains PATH header: %q", zprofileGot)
	}
}

func TestPerformUninstallStripsShellHookBlocksEvenWhenNoExecutableFound(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_INSTALL_DIR", "")

	zshrc := filepath.Join(homeDir, ".zshrc")
	content := strings.Join([]string{
		workspaceDetectionHeader,
		"proofboard detect 2>/dev/null",
		autocompletionHeader,
		"source <(proofboard completion zsh)",
	}, "\n")
	if err := os.WriteFile(zshrc, []byte(content), 0o644); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	var out strings.Builder
	if err := performUninstall(&out); err != nil {
		t.Fatalf("performUninstall: %v", err)
	}

	got, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if strings.Contains(string(got), workspaceDetectionHeader) || strings.Contains(string(got), autocompletionHeader) {
		t.Errorf("shell hook blocks remained after uninstall with no executable found: %q", got)
	}
}
