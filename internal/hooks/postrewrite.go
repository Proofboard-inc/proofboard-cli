package hooks

import (
	"context"
	"fmt"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func PostRewrite(ctx context.Context, repo pbgit.Repo, lastHead string) (bool, error) {
	head, err := pbgit.Head(ctx, repo)
	if err != nil {
		return false, fmt.Errorf("check post-rewrite head: %w", err)
	}
	return head != "" && head != lastHead, nil
}
