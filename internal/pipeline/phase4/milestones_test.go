package phase4

import (
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestDetectClustering(t *testing.T) {
	// Base time for commits
	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	// Create commits with specific categories, impacts, and timestamps
	commits := []model.CommitSignal{
		{
			SHA:             "sha1",
			Timestamp:       baseTime, // Unix: 1780324800
			Additions:       10,
			Deletions:       5,
			CategoryScores:  map[string]int{"Payments": 5, "Auth": 2},
			PrimaryCategory: "Payments",
			ImpactType:      "feature",
		},
		{
			SHA:             "sha2",
			Timestamp:       baseTime.Add(2 * time.Hour), // Unix: 1780332000
			Additions:       20,
			Deletions:       10,
			CategoryScores:  map[string]int{"Payments": 2, "Auth": 8},
			PrimaryCategory: "Auth",
			ImpactType:      "refactor",
		},
		{
			SHA:             "sha3",
			Timestamp:       baseTime.Add(24 * time.Hour), // Unix: 1.8.511200
			Additions:       5,
			Deletions:       0,
			CategoryScores:  map[string]int{"Database": 4},
			PrimaryCategory: "Database",
			ImpactType:      "bugfix",
		},
		{
			SHA:             "sha4",
			Timestamp:       baseTime.Add(48 * time.Hour), // Unix: 1.8.597600
			Additions:       15,
			Deletions:       5,
			CategoryScores:  map[string]int{"Database": 10},
			PrimaryCategory: "Database",
			ImpactType:      "bugfix",
		},
	}

	result := model.ScoredResult{
		Commits: commits,
	}

	t.Run("No merge timestamps (single cluster)", func(t *testing.T) {
		clusters := Detect(result, nil)
		if len(clusters) != 1 {
			t.Fatalf("expected 1 cluster, got %d", len(clusters))
		}

		c := clusters[0]
		// Auth (2+8=10) vs Payments (5+2=7) vs Database (4+10=14)
		// Highest sum of scores is Database (14), followed by Auth (10).
		if c.Category != "Database" {
			t.Errorf("expected Category 'Database', got %q", c.Category)
		}
		// bugfix (2) vs feature (1) vs refactor (1)
		if c.ImpactType != "bugfix" {
			t.Errorf("expected ImpactType 'bugfix', got %q", c.ImpactType)
		}
		if c.ImpactScale != "small" {
			t.Errorf("expected ImpactScale 'small', got %q", c.ImpactScale)
		}
		if c.CommitCount != 4 {
			t.Errorf("expected CommitCount 4, got %d", c.CommitCount)
		}
		if c.TotalAdditions != 50 {
			t.Errorf("expected TotalAdditions 50, got %d", c.TotalAdditions)
		}
		if c.TotalDeletions != 20 {
			t.Errorf("expected TotalDeletions 20, got %d", c.TotalDeletions)
		}
		if c.StartTimestamp != baseTime.Unix() {
			t.Errorf("expected StartTimestamp %d, got %d", baseTime.Unix(), c.StartTimestamp)
		}
		if c.EndTimestamp != baseTime.Add(48*time.Hour).Unix() {
			t.Errorf("expected EndTimestamp %d, got %d", baseTime.Add(48*time.Hour).Unix(), c.EndTimestamp)
		}
	})

	t.Run("With merge boundary", func(t *testing.T) {
		// Boundary timestamp at baseTime + 12 hours (between sha2 and sha3)
		boundary := baseTime.Add(12 * time.Hour).Unix()
		clusters := Detect(result, []int64{boundary})

		if len(clusters) != 2 {
			t.Fatalf("expected 2 clusters, got %d", len(clusters))
		}

		// First cluster: sha1, sha2
		c1 := clusters[0]
		// Payments: 5+2=7, Auth: 2+8=10 -> Category: Auth
		if c1.Category != "Auth" {
			t.Errorf("cluster 1: expected Category 'Auth', got %q", c1.Category)
		}
		// feature vs refactor (tie, alphabetically 'feature' < 'refactor')
		if c1.ImpactType != "feature" {
			t.Errorf("cluster 1: expected ImpactType 'feature', got %q", c1.ImpactType)
		}
		if c1.CommitCount != 2 {
			t.Errorf("cluster 1: expected CommitCount 2, got %d", c1.CommitCount)
		}
		if c1.TotalAdditions != 30 {
			t.Errorf("cluster 1: expected TotalAdditions 30, got %d", c1.TotalAdditions)
		}
		if c1.TotalDeletions != 15 {
			t.Errorf("cluster 1: expected TotalDeletions 15, got %d", c1.TotalDeletions)
		}
		if c1.StartTimestamp != baseTime.Unix() {
			t.Errorf("cluster 1: expected StartTimestamp %d, got %d", baseTime.Unix(), c1.StartTimestamp)
		}
		if c1.EndTimestamp != baseTime.Add(2*time.Hour).Unix() {
			t.Errorf("cluster 1: expected EndTimestamp %d, got %d", baseTime.Add(2*time.Hour).Unix(), c1.EndTimestamp)
		}

		// Second cluster: sha3, sha4
		c2 := clusters[1]
		if c2.Category != "Database" {
			t.Errorf("cluster 2: expected Category 'Database', got %q", c2.Category)
		}
		if c2.ImpactType != "bugfix" {
			t.Errorf("cluster 2: expected ImpactType 'bugfix', got %q", c2.ImpactType)
		}
		if c2.CommitCount != 2 {
			t.Errorf("cluster 2: expected CommitCount 2, got %d", c2.CommitCount)
		}
		if c2.TotalAdditions != 20 {
			t.Errorf("cluster 2: expected TotalAdditions 20, got %d", c2.TotalAdditions)
		}
		if c2.TotalDeletions != 5 {
			t.Errorf("cluster 2: expected TotalDeletions 5, got %d", c2.TotalDeletions)
		}
		if c2.StartTimestamp != baseTime.Add(24*time.Hour).Unix() {
			t.Errorf("cluster 2: expected StartTimestamp %d, got %d", baseTime.Add(24*time.Hour).Unix(), c2.StartTimestamp)
		}
		if c2.EndTimestamp != baseTime.Add(48*time.Hour).Unix() {
			t.Errorf("cluster 2: expected EndTimestamp %d, got %d", baseTime.Add(48*time.Hour).Unix(), c2.EndTimestamp)
		}
	})
}

func TestDetectSplitsLargeUnmergedHistoryIntoFourClusters(t *testing.T) {
	t.Parallel()
	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	commits := make([]model.CommitSignal, 0, 5)
	for i := 0; i < 5; i++ {
		commits = append(commits, model.CommitSignal{
			SHA:             string(rune('a' + i)),
			Timestamp:       baseTime.Add(time.Duration(i) * time.Hour),
			Additions:       10 + i,
			Deletions:       i,
			CategoryScores:  map[string]int{"Feature Development": 10 + i},
			PrimaryCategory: "Feature Development",
			ImpactType:      "feature",
		})
	}

	clusters := Detect(model.ScoredResult{Commits: commits}, nil)
	if len(clusters) != 4 {
		t.Fatalf("expected 4 clusters, got %d", len(clusters))
	}
	if clusters[0].CommitCount != 2 {
		t.Fatalf("expected first cluster to contain 2 commits, got %d", clusters[0].CommitCount)
	}
	if clusters[3].CommitCount != 1 {
		t.Fatalf("expected last cluster to contain 1 commit, got %d", clusters[3].CommitCount)
	}
}
