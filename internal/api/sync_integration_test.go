package api_test

import (
	"context"

	"os"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/config"
	"github.com/proofboard/proofboard/internal/model"
)

func TestIntegrationPayloadContract(t *testing.T) {
	token := os.Getenv("PROOFBOARD_TEST_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test; PROOFBOARD_TEST_TOKEN not set")
	}
	baseURL := os.Getenv("PROOFBOARD_API_BASE_URL")
	if baseURL == "" {
		baseURL = config.DefaultAPIBaseURL
	}

	client := api.NewClient(baseURL, "/api/v1/cli/repos/link", "/api/v1/cli/repos/check", "/api/v1/cli/sync")

	payload := model.SyncPayload{
		SHAs:         []string{"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"},
		Timestamps:   []int64{time.Now().Unix()},
		Additions:    []int{10},
		Deletions:    []int{5},
		FilesChanged: []int{2},
		Categories:   []string{"Feature Development"},
		ImpactScores: model.ImpactScores{
			Feature:     1.0,
			Bugfix:      0.0,
			Refactor:    0.0,
			Ship:        0.0,
			Maintenance: 0.0,
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
		CapturedAt:        "2026-06-15T12:00:00Z",
		CLIVersion:        "1.8.5",
		DictionaryVersion: "2.0.0",
		NotifyPush:        false,
		AntiFraudSignals: model.AntiFraudSignals{
			AINoiseScore:        0.1,
			LowCommitCount:      false,
			SingleCommitRepoCap: false,
		},
	}

	_, err := client.Sync(context.Background(), token, payload)
	if err != nil {
		t.Fatalf("Sync payload contract failed: %v", err)
	}
}
