package git

import (
	"context"
	"os/exec"
	"testing"
)

func initAuthorRepo(t *testing.T, userEmail string, commitAuthors []string) string {
	t.Helper()
	dir := t.TempDir()
	setup := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", userEmail},
	}
	for _, args := range setup {
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
	for i, author := range commitAuthors {
		cmd := exec.Command("git", "commit", "--allow-empty",
			"-m", "commit",
			"--author", "Author <"+author+">")
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit %d by %s: %v: %s", i, author, err, output)
		}
	}
	return dir
}

func TestIsSoleAuthorTrueWhenOnlyAuthorMatchesConfiguredEmail(t *testing.T) {
	dir := initAuthorRepo(t, "solo@example.com", []string{"solo@example.com", "solo@example.com"})
	got, err := IsSoleAuthor(context.Background(), Repo{Path: dir})
	if err != nil {
		t.Fatalf("IsSoleAuthor() error = %v", err)
	}
	if !got {
		t.Error("IsSoleAuthor() = false, want true for a single-author repo matching git config user.email")
	}
}

func TestIsSoleAuthorFalseWhenMultipleAuthors(t *testing.T) {
	dir := initAuthorRepo(t, "solo@example.com", []string{"solo@example.com", "other@example.com"})
	got, err := IsSoleAuthor(context.Background(), Repo{Path: dir})
	if err != nil {
		t.Fatalf("IsSoleAuthor() error = %v", err)
	}
	if got {
		t.Error("IsSoleAuthor() = true, want false when history has more than one distinct author")
	}
}

func TestIsSoleAuthorFalseWhenSingleAuthorDoesNotMatchConfiguredEmail(t *testing.T) {
	// The configured user.email differs from the single author on every
	// commit (e.g. a repo cloned from someone else, or an email changed
	// after the fact) — must not be treated as the local user's own work.
	dir := initAuthorRepo(t, "someone-else@example.com", []string{"original-author@example.com"})
	got, err := IsSoleAuthor(context.Background(), Repo{Path: dir})
	if err != nil {
		t.Fatalf("IsSoleAuthor() error = %v", err)
	}
	if got {
		t.Error("IsSoleAuthor() = true, want false when the sole author doesn't match git config user.email")
	}
}

func TestIsSoleAuthorFalseWhenNoCommits(t *testing.T) {
	dir := initAuthorRepo(t, "solo@example.com", nil)
	got, err := IsSoleAuthor(context.Background(), Repo{Path: dir})
	if err != nil {
		t.Fatalf("IsSoleAuthor() error = %v", err)
	}
	if got {
		t.Error("IsSoleAuthor() = true, want false for a repo with zero commits")
	}
}
