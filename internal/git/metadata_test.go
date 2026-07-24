package git

import (
	"context"
	"os/exec"
	"testing"
)

func TestMetadataFingerprintChangesWithRemoteRefs(t *testing.T) {
	repoDir := initMetadataRepo(t)
	repo := Repo{Path: repoDir}
	ctx := context.Background()

	first, err := MetadataFingerprint(ctx, repo)
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}
	head, err := Head(ctx, repo)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/main", head)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update remote ref: %v: %s", err, output)
	}
	second, err := MetadataFingerprint(ctx, repo)
	if err != nil {
		t.Fatalf("second fingerprint: %v", err)
	}
	if first == second {
		t.Fatal("expected metadata fingerprint to change")
	}
}

func initMetadataRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	commands := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "remote", "add", "origin", "https://github.com/example/project.git"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			if args[1] == "init" {
				fallback := exec.Command("git", "init")
				fallback.Dir = dir
				if fallbackOutput, fallbackErr := fallback.CombinedOutput(); fallbackErr == nil {
					continue
				} else {
					t.Fatalf("git init: %v: %s", fallbackErr, fallbackOutput)
				}
			}
			t.Fatalf("%v: %v: %s", args, err, output)
		}
	}
	return dir
}
