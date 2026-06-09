package git

import "testing"

func TestParseLogParsesNumstatWithoutDiffs(t *testing.T) {
	t.Parallel()
	input := []byte("\x1eabc\x1fDev@Example.com\x1f1710000000\x1fadd auth token\n10\t2\tauth/token.go\n-\t-\tassets/logo.png\n")
	commits, err := ParseLog(input)
	if err != nil {
		t.Fatalf("ParseLog returned error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	commit := commits[0]
	if commit.SHA != "abc" || string(commit.Subject) != "add auth token" {
		t.Fatalf("unexpected commit parsed: %#v", commit)
	}
	if commit.Additions != 10 || commit.Deletions != 2 || commit.FilesChanged != 2 {
		t.Fatalf("unexpected numstat totals: additions=%d deletions=%d files=%d", commit.Additions, commit.Deletions, commit.FilesChanged)
	}
}
