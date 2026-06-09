package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

const hookFileMode os.FileMode = 0o755

func Install(ctx context.Context, repo pbgit.Repo) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("install hooks: %w", err)
	}
	if err := writeHook(repo, "post-merge", "#!/bin/sh\nproofboard sync --incremental --from-hook 2>/dev/null &\n"); err != nil {
		return err
	}
	if err := writeHook(repo, "post-rewrite", "#!/bin/sh\nproofboard sync --incremental --from-hook 2>/dev/null &\n"); err != nil {
		return err
	}
	return nil
}

func writeHook(repo pbgit.Repo, name string, content string) error {
	path := filepath.Join(repo.Path, ".git", "hooks", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), hookFileMode); err != nil {
		return fmt.Errorf("write %s hook: %w", name, err)
	}
	return nil
}
