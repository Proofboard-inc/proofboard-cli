package git

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

type Handshake struct {
	RemoteHash string
}

func LSRemoteHandshake(ctx context.Context, repo Repo, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "ls-remote", "--exit-code", "origin", "HEAD")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("remote handshake timed out: %w", ctx.Err())
		}
		return fmt.Errorf("remote handshake failed: %w", err)
	}
	return nil
}
