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
		AccessStatus:      "unknown",
		CapturedAt:        time.Now().UTC().Format(time.RFC3339),
		CLIVersion:        input.CLIVersion,
		DictionaryVersion: input.DictionaryVersion,
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
		payload.Categories = append(payload.Categories, commit.Category)
		impactCounts[commit.ImpactType]++
		totalNoise += commit.NoiseScore
		if input.EmailHash != "" && commit.AuthorEmailHash != "" && commit.AuthorEmailHash != input.EmailHash {
			identityMismatch++
		}
	}

	payload.AntiFraudSignals.IdentityMismatch = identityMismatch
	
	signedCount := 0
	var lastTime int64
	var intervals []float64
	var hours []float64
	burstCount := 0

	for i, commit := range input.Commits {
		if commit.SignatureValid {
			payload.AntiFraudSignals.CommitSignatureVerified = true
			signedCount++
		}
		
		t := time.Unix(commit.TimestampUnix, 0).UTC()
		hours = append(hours, float64(t.Hour()))
		
		if i > 0 {
			diff := commit.TimestampUnix - lastTime
			if diff < 0 {
				diff = -diff
			}
			intervals = append(intervals, float64(diff))
			if diff < 300 {
				burstCount++
			}
		}
		lastTime = commit.TimestampUnix
	}
	
	if len(input.Commits) > 0 {
		payload.AntiFraudSignals.SignedCommitRatio = float64(signedCount) / float64(len(input.Commits))
		if len(input.Commits) > 1 {
			payload.AntiFraudSignals.BurstPatternScore = float64(burstCount) / float64(len(input.Commits)-1)
			
			// interval variance
			var sumInterval float64
			for _, v := range intervals {
				sumInterval += v
			}
			meanInterval := sumInterval / float64(len(intervals))
			var varInterval float64
			for _, v := range intervals {
				diff := v - meanInterval
				varInterval += diff * diff
			}
			payload.AntiFraudSignals.CommitIntervalVariance = varInterval / float64(len(intervals))
		}
		
		// time of day dist (stddev of hours)
		var sumHour float64
		for _, v := range hours {
			sumHour += v
		}
		meanHour := sumHour / float64(len(hours))
		var varHour float64
		for _, v := range hours {
			diff := v - meanHour
			varHour += diff * diff
		}
		// instead of stddev, we can just put variance or scaled stddev. Let's do variance.
		payload.AntiFraudSignals.TimeOfDayDistribution = varHour / float64(len(hours))
	}
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
