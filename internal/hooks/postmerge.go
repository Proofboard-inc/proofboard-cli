package hooks

import (
	"context"
	"fmt"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func PostMerge(ctx context.Context, repo pbgit.Repo, productionBranches []string) (bool, error) {
	branch, err := pbgit.CurrentBranch(ctx, repo)
	if err != nil {
		return false, fmt.Errorf("check post-merge branch: %w", err)
	}
	return pbgit.IsProductionBranch(branch, productionBranches), nil
}
