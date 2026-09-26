package git

import (
	"context"
	"os/exec"
	"strings"
)

// IsSoleAuthor reports whether every commit in the repository's history was
// authored by the single email address configured as git config user.email.
// Used by the ownership wizard's personal-project branch to decide whether
// "Owner" is a sensible default suggestion for the role/contributor prompt:
// a repo with exactly one distinct author, matching the local user, is very
// likely a solo personal project rather than a fork/clone of someone else's
// work or a repo with other contributors.
//
// Deliberately conservative: any read/parse failure, an empty configured
// email, zero commits, or more than one distinct author all report false
// rather than guessing.
func IsSoleAuthor(ctx context.Context, repo Repo) (bool, error) {
	email, err := UserEmail(ctx, repo)
	if err != nil {
		return false, err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return false, nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "log", "--format=%ae")
	out, err := cmd.Output()
	if err != nil {
		// A repository with zero commits (no HEAD yet) makes `git log` exit
		// non-zero — that is a legitimate state, not a failure worth
		// surfacing, and it means there is no author history to suggest
		// "Owner" from.
		return false, nil
	}

	authors := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		authors[strings.ToLower(line)] = true
	}
	if len(authors) != 1 {
		return false, nil
	}
	for author := range authors {
		return author == strings.ToLower(email), nil
	}
	return false, nil
}
