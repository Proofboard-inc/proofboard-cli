package pipeline

import (
	"context"
	"fmt"

	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/pipeline/phase2"
	"github.com/proofboard/proofboard/internal/pipeline/phase3"
	"github.com/proofboard/proofboard/internal/pipeline/phase4"
	"github.com/proofboard/proofboard/internal/pipeline/phase5"
	"github.com/proofboard/proofboard/internal/pipeline/phase7"
	"github.com/proofboard/proofboard/internal/version"
)

type Pipeline struct {
	dictionary model.Dictionary
}

func New(dictionary model.Dictionary) Pipeline {
	return Pipeline{dictionary: dictionary}
}

type RunInput struct {
	Raw             []model.RawCommit
	OrgHash         string
	RepoHash        string
	EmailHash       string
	Provider        string
	ExpectedOrgHash string
	MergeTimestamps []int64
}

func (p Pipeline) Run(ctx context.Context, input RunInput) (model.SyncPayload, error) {
	if err := ctx.Err(); err != nil {
		return model.SyncPayload{}, fmt.Errorf("run pipeline: %w", err)
	}
	classified := phase2.Classify(input.Raw, p.dictionary)
	scored := phase3.Score(classified, p.dictionary.Version)
	clusters := phase4.Detect(scored, input.MergeTimestamps)
	safe := phase5.Shred(input.Raw, classified)
	payload := phase7.Assemble(phase7.AssemblyInput{
		Commits:           safe,
		Clusters:          clusters,
		OrgHash:           input.OrgHash,
		RepoHash:          input.RepoHash,
		EmailHash:         input.EmailHash,
		Provider:          input.Provider,
		CLIVersion:        version.Version,
		DictionaryVersion: p.dictionary.Version,
		ExpectedOrgHash:   input.ExpectedOrgHash,
	})
	return payload, nil
}
