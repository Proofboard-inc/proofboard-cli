package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Repo struct {
	Path string
}

func Discover(ctx context.Context, dir string) (Repo, error) {
	if err := ctx.Err(); err != nil {
		return Repo{}, fmt.Errorf("discover git repo: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return Repo{}, fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return Repo{Path: strings.TrimSpace(string(out))}, nil
}

func Head(ctx context.Context, repo Repo) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func UserEmail(ctx context.Context, repo Repo) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "config", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git config user.email: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
