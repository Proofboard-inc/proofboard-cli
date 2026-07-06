package api_test

import (
	"context"

	"os"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/model"
)

func TestIntegrationPayloadContract(t *testing.T) {
	token := os.Getenv("PROOFBOARD_TEST_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test; PROOFBOARD_TEST_TOKEN not set")
	}
	baseURL := os.Getenv("PROOFBOARD_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api-dev.proofboard.io"
	}

	client := api.NewClient(baseURL, "/api/v1/cli/repos/link", "/api/v1/cli/sync")

	payload := model.SyncPayload{
		SHAs:              []string{"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"},
		Timestamps:        []int64{time.Now().Unix()},
		Additions:         []int{10},
		Deletions:         []int{5},
		FilesChanged:      []int{2},
		Categories:        []string{"Feature Development"},
		ImpactScores: map[string]float64{
			"feature":     1.0,
			"bugfix":      0.0,
			"refactor":    0.0,
			"ship":        0.0,
			"maintenance": 0.0,
		},
		MilestoneClusters: []model.Cluster{
			{
				ClusterIndex:      0,
				Category:          "Feature Development",
				CommitCount:       1,
				StartTimestamp:    time.Now().Unix() - 3600,
				EndTimestamp:      time.Now().Unix(),
				TotalAdditions:    10,
				TotalDeletions:    5,
				TotalFilesChanged: 2,
				ImpactType:        "feature",
				ImpactScale:       "small",
			},
		},
		OrgHash:           "dummy-org-hash",
		RepoHash:          "dummy-repo-hash",
		EmailHash:         "dummy-email-hash",
		Provider:          "github",
		AccessStatus:      "unknown",
		CapturedAt:        "2026-06-15T12:00:00Z",
		CLIVersion:        "1.8.0",
		DictionaryVersion: "2.0.0",
		AntiFraudSignals: model.AntiFraudSignals{
			AINoiseScore:            0.1,
			OrgHashMismatch:         false,
			IdentityMismatch:        0,
			LowCommitCount:          false,
			SingleCommitRepoCap:     false,
			CommitSignatureVerified: true,
			SignedCommitRatio:       1.0,
			CommitIntervalVariance:  100.0,
			TimeOfDayDistribution:   0.5,
			BurstPatternScore:       0.1,
		},
	}

	_, err := client.Sync(context.Background(), token, payload)
	if err != nil {
		t.Fatalf("Sync payload contract failed: %v", err)
	}
}
