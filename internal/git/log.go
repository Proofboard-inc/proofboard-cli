package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sort"
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
	args = append(args, "--format=%x1e%H%x1f%ae%x1f%at%x1f%G?%x1f%s", "--numstat", "--no-merges", "--author="+email)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return ParseLog(out)
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

