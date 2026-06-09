package phase1

import (
	"context"
	"fmt"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
)

func Ingest(ctx context.Context, repo pbgit.Repo, lastSHA string) ([]model.RawCommit, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("ingest git history: %w", err)
	}
	commits, err := pbgit.Log(ctx, repo, lastSHA)
	if err != nil {
		return nil, fmt.Errorf("read git log: %w", err)
	}
	return commits, nil
}
