package phase7

import (
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

type AssemblyInput struct {
	Commits           []model.SafeCommit
	Clusters          []model.Cluster
	OrgHash           string
	RepoHash          string
	EmailHash         string
	HandshakeStatus   string
	CLIVersion        string
	DictionaryVersion string
	ExpectedOrgHash   string
}

func Assemble(input AssemblyInput) model.SyncPayload {
	payload := model.SyncPayload{
		SHAs:              make([]string, 0, len(input.Commits)),
		Timestamps:        make([]int64, 0, len(input.Commits)),
		Additions:         make([]int, 0, len(input.Commits)),
		Deletions:         make([]int, 0, len(input.Commits)),
		FilesChanged:      make([]int, 0, len(input.Commits)),
		Categories:        make([]string, 0, len(input.Commits)),
		ImpactScores:      make(map[string]int),
		MilestoneClusters: input.Clusters,
		OrgHash:           input.OrgHash,
		RepoHash:          input.RepoHash,
		EmailHash:         input.EmailHash,
		HandshakeStatus:   input.HandshakeStatus,
		CapturedAt:        time.Now().UTC(),
		CLIVersion:        input.CLIVersion,
		DictionaryVersion: input.DictionaryVersion,
		LowCommitCount:    len(input.Commits) < 5,
		OrgHashMismatch:   input.ExpectedOrgHash != "" && input.ExpectedOrgHash != input.OrgHash,
	}
	totalNoise := 0.0
	for _, commit := range input.Commits {
		payload.SHAs = append(payload.SHAs, commit.SHA)
		payload.Timestamps = append(payload.Timestamps, commit.TimestampUnix)
		payload.Additions = append(payload.Additions, commit.Additions)
		payload.Deletions = append(payload.Deletions, commit.Deletions)
		payload.FilesChanged = append(payload.FilesChanged, commit.FilesChanged)
		payload.Categories = append(payload.Categories, commit.Category)
		payload.ImpactScores[commit.ImpactType]++
		totalNoise += commit.NoiseScore
		if input.EmailHash != "" && commit.AuthorEmailHash != "" && commit.AuthorEmailHash != input.EmailHash {
			payload.IdentityMismatch++
		}
	}
	if len(input.Commits) > 0 {
		payload.AINoiseScore = totalNoise / float64(len(input.Commits))
	}
	return payload
}
