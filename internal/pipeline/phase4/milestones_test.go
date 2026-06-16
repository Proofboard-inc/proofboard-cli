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
			Timestamp:       baseTime.Add(24 * time.Hour), // Unix: 1780411200
			Additions:       5,
			Deletions:       0,
			CategoryScores:  map[string]int{"Database": 4},
			PrimaryCategory: "Database",
			ImpactType:      "bugfix",
		},
		{
			SHA:             "sha4",
			Timestamp:       baseTime.Add(48 * time.Hour), // Unix: 1780497600
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
		if c.ClusterLabel != "Database" {
			t.Errorf("expected ClusterLabel 'Database', got %q", c.ClusterLabel)
		}
		// bugfix (2) vs feature (1) vs refactor (1)
		if c.ImpactType != "bugfix" {
			t.Errorf("expected ImpactType 'bugfix', got %q", c.ImpactType)
		}
		if c.Scale != "small" {
			t.Errorf("expected Scale 'small', got %q", c.Scale)
		}
		if c.CommitCount != 4 {
			t.Errorf("expected CommitCount 4, got %d", c.CommitCount)
		}
		if c.AdditionTotal != 50 {
			t.Errorf("expected AdditionTotal 50, got %d", c.AdditionTotal)
		}
		if c.DeletionTotal != 20 {
			t.Errorf("expected DeletionTotal 20, got %d", c.DeletionTotal)
		}
		if c.DurationDays != 2 { // diff between sha4 and sha1 is 48 hours = 2 days
			t.Errorf("expected DurationDays 2, got %d", c.DurationDays)
		}
		if len(c.ReferenceSHABucket) != 3 {
			t.Errorf("expected 3 reference SHAs, got %d", len(c.ReferenceSHABucket))
		}
		expectedSHAs := []string{"sha1", "sha2", "sha3"}
		for i, sha := range expectedSHAs {
			if c.ReferenceSHABucket[i] != sha {
				t.Errorf("expected ReferenceSHA[%d] = %q, got %q", i, sha, c.ReferenceSHABucket[i])
			}
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
		// Payments: 5+2=7, Auth: 2+8=10 -> ClusterLabel: Auth
		if c1.ClusterLabel != "Auth" {
			t.Errorf("cluster 1: expected ClusterLabel 'Auth', got %q", c1.ClusterLabel)
		}
		// feature vs refactor (tie, alphabetically 'feature' < 'refactor')
		if c1.ImpactType != "feature" {
			t.Errorf("cluster 1: expected ImpactType 'feature', got %q", c1.ImpactType)
		}
		if c1.CommitCount != 2 {
			t.Errorf("cluster 1: expected CommitCount 2, got %d", c1.CommitCount)
		}
		if c1.AdditionTotal != 30 {
			t.Errorf("cluster 1: expected AdditionTotal 30, got %d", c1.AdditionTotal)
		}
		if c1.DeletionTotal != 15 {
			t.Errorf("cluster 1: expected DeletionTotal 15, got %d", c1.DeletionTotal)
		}
		if c1.DurationDays != 0 { // diff is 2 hours = 0 days
			t.Errorf("cluster 1: expected DurationDays 0, got %d", c1.DurationDays)
		}

		// Second cluster: sha3, sha4
		c2 := clusters[1]
		if c2.ClusterLabel != "Database" {
			t.Errorf("cluster 2: expected ClusterLabel 'Database', got %q", c2.ClusterLabel)
		}
		if c2.ImpactType != "bugfix" {
			t.Errorf("cluster 2: expected ImpactType 'bugfix', got %q", c2.ImpactType)
		}
		if c2.CommitCount != 2 {
			t.Errorf("cluster 2: expected CommitCount 2, got %d", c2.CommitCount)
		}
		if c2.AdditionTotal != 20 {
			t.Errorf("cluster 2: expected AdditionTotal 20, got %d", c2.AdditionTotal)
		}
		if c2.DeletionTotal != 5 {
			t.Errorf("cluster 2: expected DeletionTotal 5, got %d", c2.DeletionTotal)
		}
		if c2.DurationDays != 1 { // diff between sha3 and sha4 is 24 hours = 1 day
			t.Errorf("cluster 2: expected DurationDays 1, got %d", c2.DurationDays)
		}
	})
}
