package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/state"
)

func TestDetectCommandIsSilentAndLogsWorkspaceState(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", tempHome)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# proofboard\n"), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "README.md").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "feat: initial commit").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	ctx := context.Background()
	var out bytes.Buffer
	cmd := newDetectCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("detect command failed: %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("expected silent detect command, got output: %q", out.String())
	}

	st, err := state.NewStore(tempHome).Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repoHash := crypto.SHA256("github:org/repo")
	if _, ok := st.LinkedRepos[repoHash]; ok {
		t.Fatalf("expected detect to avoid mutating linked repo state")
	}

	logPath := filepath.Join(tempHome, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read sync log: %v", err)
	}
	logContent := string(data)
	if !strings.Contains(logContent, "detect") {
		t.Fatalf("expected detect entry in sync.log, got: %s", logContent)
	}
	if !strings.Contains(logContent, "link") {
		t.Fatalf("expected detect action to be logged, got: %s", logContent)
	}
}

func TestDetectCommandJsonOutputsInspection(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", tempHome)

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# proofboard\n"), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "README.md").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "feat: initial commit").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	ctx := context.Background()
	var out bytes.Buffer
	cmd := newDetectCommand(ctx, &out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("detect command failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal detect output: %v; output=%q", err, out.String())
	}
	if got := result["action"]; got != "link" {
		t.Fatalf("expected action link, got %v", got)
	}
	if got := result["repoName"]; got != "repo" {
		t.Fatalf("expected repo name repo, got %v", got)
	}
	if got := result["suggestedAction"]; got != "Sync Project" {
		t.Fatalf("expected Career Agent action Sync Project, got %v", got)
	}
	if _, exists := result["suggestedCommand"]; exists {
		t.Fatal("detect output exposed a CLI command as the primary action")
	}
}
