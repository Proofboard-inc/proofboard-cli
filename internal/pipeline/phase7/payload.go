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
	Provider          string
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
		ImpactScores:      make(map[string]float64),
		MilestoneClusters: input.Clusters,
		OrgHash:           input.OrgHash,
		RepoHash:          input.RepoHash,
		EmailHash:         input.EmailHash,
		Provider:          input.Provider,
		CapturedAt:        time.Now().UTC().Format(time.RFC3339),
		CLIVersion:        input.CLIVersion,
		DictionaryVersion: input.DictionaryVersion,
		NotifyPush:        false,
		AntiFraudSignals: model.AntiFraudSignals{
			LowCommitCount:      len(input.Commits) < 5,
			OrgHashMismatch:     input.ExpectedOrgHash != "" && input.ExpectedOrgHash != input.OrgHash,
			SingleCommitRepoCap: false,
		},
	}
	
	impactCounts := make(map[string]int)
	totalNoise := 0.0
	identityMismatch := 0

	for _, commit := range input.Commits {
		payload.SHAs = append(payload.SHAs, commit.SHA)
		payload.Timestamps = append(payload.Timestamps, commit.TimestampUnix)
		payload.Additions = append(payload.Additions, commit.Additions)
		payload.Deletions = append(payload.Deletions, commit.Deletions)
		payload.FilesChanged = append(payload.FilesChanged, commit.FilesChanged)
		cat := commit.Category
		if cat == "Unclassified" {
			cat = "Feature Development"
		}
		payload.Categories = append(payload.Categories, cat)
		impactCounts[commit.ImpactType]++
		totalNoise += commit.NoiseScore
		if input.EmailHash != "" && commit.AuthorEmailHash != "" && commit.AuthorEmailHash != input.EmailHash {
			identityMismatch++
		}
	}

	payload.AntiFraudSignals.IdentityMismatch = identityMismatch
	

	if len(input.Commits) > 0 {
		payload.AntiFraudSignals.AINoiseScore = totalNoise / float64(len(input.Commits))
		total := float64(len(input.Commits))
		for _, typ := range []string{"feature", "bugfix", "refactor", "ship", "maintenance"} {
			payload.ImpactScores[typ] = float64(impactCounts[typ]) / total
		}
	} else {
		for _, typ := range []string{"feature", "bugfix", "refactor", "ship", "maintenance"} {
			payload.ImpactScores[typ] = 0.0
		}
	}

	return payload
}
