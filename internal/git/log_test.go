package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
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

func TestFilterCommitsByAuthorEmailRequiresExactNormalizedIdentity(t *testing.T) {
	t.Parallel()
	commits := []model.RawCommit{
		{SHA: "own-lower", AuthorEmail: "dev@example.com"},
		{SHA: "own-case", AuthorEmail: " Dev@Example.com "},
		{SHA: "lookalike-prefix", AuthorEmail: "dev@example.com.attacker.test"},
		{SHA: "lookalike-suffix", AuthorEmail: "other+dev@example.com"},
		{SHA: "other", AuthorEmail: "other@example.com"},
	}

	filtered := FilterCommitsByAuthorEmail(commits, " DEV@example.com ")
	if len(filtered) != 2 {
		t.Fatalf("filtered commits = %#v, want exactly two local-identity commits", filtered)
	}
	if filtered[0].SHA != "own-lower" || filtered[1].SHA != "own-case" {
		t.Fatalf("filtered SHAs = %q, %q", filtered[0].SHA, filtered[1].SHA)
	}
	if got := FilterCommitsByAuthorEmail(commits, " "); got != nil {
		t.Fatalf("empty identity returned commits: %#v", got)
	}
}

func TestQuoteGitBasicRegexpKeepsCommonEmailCharactersLiteral(t *testing.T) {
	t.Parallel()
	got := quoteGitBasicRegexp(`dev+mobile[ci]@example.com`)
	want := `dev+mobile\[ci]@example\.com`
	if got != want {
		t.Fatalf("quoteGitBasicRegexp() = %q, want %q", got, want)
	}
}

func TestLogReturnsOnlyExactConfiguredIdentityCommits(t *testing.T) {
	repoDir := t.TempDir()
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(ctx, "git", append([]string{"-C", repoDir}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Local Engineer")
	run("config", "user.email", "dev+mobile@example.com")
	run("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(repoDir, "own.txt"), []byte("own\n"), 0o600); err != nil {
		t.Fatalf("write own file: %v", err)
	}
	run("add", "own.txt")
	run(
		"-c", "user.email=Dev+Mobile@Example.com",
		"commit", "-m", "own commit",
	)

	if err := os.WriteFile(filepath.Join(repoDir, "other.txt"), []byte("other\n"), 0o600); err != nil {
		t.Fatalf("write other file: %v", err)
	}
	run("add", "other.txt")
	run(
		"-c", "user.name=Other Engineer",
		"-c", "user.email=dev+mobile@example.com.attacker.test",
		"commit", "-m", "other commit",
	)

	commits, err := Log(ctx, Repo{Path: repoDir}, "")
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	if len(commits) != 1 || commits[0].AuthorEmail != "Dev+Mobile@Example.com" {
		t.Fatalf("Log() returned non-local commits: %#v", commits)
	}
}

// ParseBodies unit tests covering edge cases in the STX/ETX-delimited
// body format — empty body, whitespace-only body, and a body containing
// characters that could collide with the OTHER call's delimiters (\x1e/\x1f)
// or literal newlines, none of which should confuse this parser since it uses
// distinct \x02/\x03/\x1f delimiters.
func TestParseBodiesHandlesMultilineAndEdgeCases(t *testing.T) {
	t.Parallel()

	input := []byte(
		"\x02sha1\x1fFirst line of body.\n\nSecond paragraph with details.\x03\n" +
			"\x02sha2\x1f\x03\n" + // empty body
			"\x02sha3\x1f   \x03\n" + // whitespace-only body (kept as-is; not trimmed)
			"\x02sha4\x1fBody containing \x1e and \x1f characters literally.\x03\n",
	)

	bodies := parseBodies(input)

	if got := string(bodies["sha1"]); got != "First line of body.\n\nSecond paragraph with details." {
		t.Fatalf("sha1 body = %q", got)
	}
	if _, ok := bodies["sha2"]; ok {
		t.Fatalf("sha2 (empty body) should not be present in the map, got %q", bodies["sha2"])
	}
	if got := string(bodies["sha3"]); got != "   " {
		t.Fatalf("sha3 body = %q, want whitespace preserved", got)
	}
	if got := string(bodies["sha4"]); got != "Body containing \x1e and \x1f characters literally." {
		t.Fatalf("sha4 body = %q", got)
	}
}

// Log() must attach multi-line commit bodies via the separate git log
// call without corrupting the primary header/numstat parse of adjacent
// commits — the two git log invocations are entirely independent, so a body
// with embedded blank lines must not shift what the numstat parser sees.
func TestLogAttachesMultilineBodiesWithoutCorruptingNumstat(t *testing.T) {
	repoDir := t.TempDir()
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(ctx, "git", append([]string{"-C", repoDir}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Local Engineer")
	run("config", "user.email", "dev@example.com")
	run("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	run("add", "a.txt")
	run("commit", "-m", "updates", "-m", "adds chargeCard() and imports stripe\n\nSecond paragraph.")

	if err := os.WriteFile(filepath.Join(repoDir, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	run("add", "b.txt")
	run("commit", "-m", "no body here")

	commits, err := Log(ctx, Repo{Path: repoDir}, "")
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Commits come back newest-first from `git log`.
	withBody := commits[0]
	if string(withBody.Subject) != "no body here" {
		t.Fatalf("unexpected first commit subject: %q", withBody.Subject)
	}
	if withBody.FilesChanged != 1 || len(withBody.FilePaths) != 1 || withBody.FilePaths[0] != "b.txt" {
		t.Fatalf("numstat corrupted for no-body commit: %#v", withBody)
	}

	withoutBody := commits[1]
	if string(withoutBody.Subject) != "updates" {
		t.Fatalf("unexpected second commit subject: %q", withoutBody.Subject)
	}
	if withoutBody.FilesChanged != 1 || len(withoutBody.FilePaths) != 1 || withoutBody.FilePaths[0] != "a.txt" {
		t.Fatalf("numstat corrupted for multi-line-body commit: %#v", withoutBody)
	}
	if string(withoutBody.Body) != "adds chargeCard() and imports stripe\n\nSecond paragraph.\n" {
		t.Fatalf("unexpected body: %q", withoutBody.Body)
	}
	if withBody.Body != nil {
		t.Fatalf("expected nil Body for the bodyless commit, got %q", withBody.Body)
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
	runGit("init", "--initial-branch=main")
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
