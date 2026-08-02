package git

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func Log(ctx context.Context, repo Repo, lastSHA string) ([]model.RawCommit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	email, err := UserEmail(ctx, repo)
	if err != nil {
		return nil, err
	}
	args := []string{"-C", repo.Path, "log"}
	if lastSHA != "" {
		args = append(args, lastSHA+"..HEAD")
	}
	args = append(
		args,
		"--format=%x1e%H%x1f%ae%x1f%at%x1f%G?%x1f%s",
		"--numstat",
		"--no-merges",
		"--regexp-ignore-case",
		"--author="+quoteGitBasicRegexp(email),
	)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	commits, err := ParseLog(out)
	if err != nil {
		return nil, err
	}
	commits = FilterCommitsByAuthorEmail(commits, email)

	// Fetch commit bodies via a second, separate git log call — kept
	// entirely apart from the header/numstat call above because commit
	// bodies are free-form multi-line text that could otherwise corrupt the
	// existing line-oriented numstat parser. Best-effort: a failure here
	// (e.g. an unusual repo state) must never fail an otherwise-successful
	// sync — commits simply proceed with a nil Body.
	if bodies, err := logBodies(ctx, repo, lastSHA, email); err == nil {
		attachBodies(commits, bodies)
	}

	return commits, nil
}

// logBodies fetches the full commit message body for each commit in the same
// range/author scope as the primary Log() call, keyed by SHA. Uses STX/ETX
// (\x02/\x03) record delimiters, which never appear in ordinary commit text,
// so a body containing literal newlines (unlike Subject, which is always a
// single line) can't be confused with the next record.
func logBodies(ctx context.Context, repo Repo, lastSHA, email string) (map[string][]byte, error) {
	args := []string{"-C", repo.Path, "log"}
	if lastSHA != "" {
		args = append(args, lastSHA+"..HEAD")
	}
	args = append(
		args,
		"--format=%x02%H%x1f%b%x03",
		"--no-merges",
		"--regexp-ignore-case",
		"--author="+quoteGitBasicRegexp(email),
	)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log bodies: %w", err)
	}
	return parseBodies(out), nil
}

func parseBodies(out []byte) map[string][]byte {
	bodies := make(map[string][]byte)
	records := strings.Split(string(out), "\x02")
	for _, record := range records {
		record = strings.TrimSuffix(record, "\n")
		record = strings.TrimSuffix(record, "\x03")
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x1f", 2)
		if len(parts) != 2 || parts[1] == "" {
			continue
		}
		bodies[parts[0]] = []byte(parts[1])
	}
	return bodies
}

func attachBodies(commits []model.RawCommit, bodies map[string][]byte) {
	for i := range commits {
		if body, ok := bodies[commits[i].SHA]; ok {
			commits[i].Body = body
		}
	}
}

func quoteGitBasicRegexp(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`[`, `\[`,
		`*`, `\*`,
		`^`, `\^`,
		`$`, `\$`,
	)
	return replacer.Replace(value)
}

// FilterCommitsByAuthorEmail enforces exact local identity attribution after
// Git's regex-based --author pre-filter. This keeps repositories
// provider-agnostic without allowing another contributor's commits into the
// local career record.
func FilterCommitsByAuthorEmail(commits []model.RawCommit, email string) []model.RawCommit {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	filtered := make([]model.RawCommit, 0, len(commits))
	for _, commit := range commits {
		if strings.EqualFold(strings.TrimSpace(commit.AuthorEmail), email) {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

func ParseLog(out []byte) ([]model.RawCommit, error) {
	records := strings.Split(string(out), "\x1e")
	commits := make([]model.RawCommit, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		lines := strings.Split(record, "\n")
		header := strings.SplitN(lines[0], "\x1f", 5)
		if len(header) != 5 {
			return nil, fmt.Errorf("invalid git log header")
		}
		unixSeconds, err := strconv.ParseInt(header[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse commit timestamp: %w", err)
		}
		commit := model.RawCommit{
			SHA:            header[0],
			AuthorEmail:    header[1],
			Timestamp:      time.Unix(unixSeconds, 0).UTC(),
			SignatureValid: header[3] == "G" || header[3] == "U",
			Subject:        []byte(header[4]),
		}
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			additions := parseNumstat(fields[0])
			deletions := parseNumstat(fields[1])
			commit.Additions += additions
			commit.Deletions += deletions
			commit.FilesChanged++
			commit.FilePaths = append(commit.FilePaths, strings.Join(fields[2:], " "))
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func parseNumstat(value string) int {
	if value == "-" {
		return 0
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return number
}

// MergeTimestamps returns sorted Unix timestamps of all merge commits in the repository.
func MergeTimestamps(ctx context.Context, repo Repo) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "log", "--merges", "--format=%at")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log merges: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	var timestamps []int64
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse merge timestamp %q: %w", line, err)
		}
		timestamps = append(timestamps, t)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	return timestamps, nil
}
