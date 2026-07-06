package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseLogParsesNumstatWithoutDiffs(t *testing.T) {
	t.Parallel()
	input := []byte("\x1eabc\x1fDev@Example.com\x1f1710000000\x1fG\x1fadd auth token\n10\t2\tauth/token.go\n-\t-\tassets/logo.png\n")
	commits, err := ParseLog(input)
	if err != nil {
		t.Fatalf("ParseLog returned error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	commit := commits[0]
	if commit.SHA != "abc" || string(commit.Subject) != "add auth token" || !commit.SignatureValid {
		t.Fatalf("unexpected commit parsed: %#v", commit)
	}
	if commit.Additions != 10 || commit.Deletions != 2 || commit.FilesChanged != 2 {
		t.Fatalf("unexpected numstat totals: additions=%d deletions=%d files=%d", commit.Additions, commit.Deletions, commit.FilesChanged)
	}
}

func TestMergeTimestamps(t *testing.T) {
	// Create a temp directory for git repo
	tempDir, err := os.MkdirTemp("", "proofboard-git-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Helper to run git commands in tempDir
	runGit := func(args ...string) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", tempDir}, args...)...)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command %v failed: %v", args, err)
		}
	}

	// Initialize repo
	runGit("init")
	// Configure user
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "commit.gpgsign", "false")

	// Check on empty repo - git log merges should error or return empty.
	repo := Repo{Path: tempDir}
	_, err = MergeTimestamps(ctx, repo)
	if err == nil {
		t.Error("expected error for empty repo, got nil")
	}

	// Create a commit on main
	err = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit("add", "file.txt")
	runGit("commit", "-m", "first commit")

	// Create a branch
	runGit("checkout", "-b", "branch1")
	err = os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("world"), 0644)
	if err != nil {
		t.Fatalf("write file2: %v", err)
	}
	runGit("add", "file2.txt")
	runGit("commit", "-m", "branch commit")

	// Checkout main
	runGit("checkout", "main")

	// Merge branch1 into main with a merge commit
	runGit("merge", "branch1", "--no-ff", "-m", "Merge branch1")

	// Now MergeTimestamps should return exactly 1 timestamp
	timestamps, err := MergeTimestamps(ctx, repo)
	if err != nil {
		t.Fatalf("MergeTimestamps failed: %v", err)
	}
	if len(timestamps) != 1 {
		t.Errorf("expected 1 timestamp, got %d", len(timestamps))
	}
}
