package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func Uninstall(ctx context.Context, repo pbgit.Repo) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("uninstall hooks: %w", err)
	}
	for _, hook := range []string{"post-commit", "post-merge", "post-rewrite"} {
		path := filepath.Join(repo.Path, ".git", "hooks", hook)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s hook: %w", hook, err)
		}
	}
	return nil
}
