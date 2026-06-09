package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Branch struct {
	Name string
}

func CurrentBranch(ctx context.Context, repo Repo) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func IsProductionBranch(branch string, allowed []string) bool {
	for _, candidate := range allowed {
		if branch == candidate {
			return true
		}
	}
	return false
}
