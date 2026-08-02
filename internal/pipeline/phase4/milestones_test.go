package phase4

import (
	"strconv"
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

// Linear history (no merge commits) with 5 same-category commits an hour
// apart is one cohesive work session — it must produce ONE milestone, not an
// arbitrary N-way even split. (Replaces the old force-split-into-4 behavior:
// segmenting content-agnostically into 4 chunks was itself a symptom of the
// over-fragmentation this redesign fixes.)
func TestDetectLinearSameCategoryHistoryIsOneCluster(t *testing.T) {
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
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cohesive cluster for same-category linear history, got %d", len(clusters))
	}
	if clusters[0].CommitCount != 5 {
		t.Fatalf("expected the single cluster to hold all 5 commits, got %d", clusters[0].CommitCount)
	}
}

// Linear history whose primary category shifts must segment at the category
// boundaries — "auth work, then payments work, then database work" is three
// milestones, detected without any merge-commit signal.
func TestDetectLinearHistorySegmentsByCategoryShift(t *testing.T) {
	t.Parallel()
	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	categories := []string{
		"Authentication & Security", "Authentication & Security",
		"Payments & Billing", "Payments & Billing", "Payments & Billing",
		"Database & Data Layer", "Database & Data Layer",
	}
	commits := make([]model.CommitSignal, 0, len(categories))
	for i, cat := range categories {
		commits = append(commits, model.CommitSignal{
			SHA:             "sha" + strconv.Itoa(i),
			Timestamp:       baseTime.Add(time.Duration(i) * time.Hour),
			Additions:       10,
			CategoryScores:  map[string]int{cat: 10},
			PrimaryCategory: cat,
			ImpactType:      "feature",
		})
	}

	clusters := Detect(model.ScoredResult{Commits: commits}, nil)
	if len(clusters) != 3 {
		t.Fatalf("expected 3 category-run clusters, got %d", len(clusters))
	}
	wantCats := []string{"Authentication & Security", "Payments & Billing", "Database & Data Layer"}
	wantCounts := []int{2, 3, 2}
	for i, c := range clusters {
		if c.Category != wantCats[i] {
			t.Errorf("cluster %d: category = %q, want %q", i, c.Category, wantCats[i])
		}
		if c.CommitCount != wantCounts[i] {
			t.Errorf("cluster %d: commitCount = %d, want %d", i, c.CommitCount, wantCounts[i])
		}
	}
}

// Σ cluster.CommitCount must always equal len(commits) — the backend
// relies on this to trust len(shas). groupCommits/segmentLinear/consolidate
// only ever regroup commits (never drop them), so every commit lands in
// exactly one retained cluster. This catches a future change that breaks that
// invariant, and also asserts no cluster is ever empty and the count never
// exceeds the per-sync cap.
func TestDetectCommitCountInvariant(t *testing.T) {
	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	sizes := []int{0, 1, 3, 4, 5, 10, 30, 137}
	for _, n := range sizes {
		n := n
		t.Run(sizeName(n), func(t *testing.T) {
			commits := make([]model.CommitSignal, n)
			for i := 0; i < n; i++ {
				commits[i] = model.CommitSignal{
					SHA:             sha(i),
					Timestamp:       baseTime.Add(time.Duration(i) * time.Hour),
					Additions:       1,
					PrimaryCategory: "Feature",
					ImpactType:      "feature",
				}
			}

			// Half the sizes get no merge timestamps (pure rebalance path),
			// half get a few interspersed merge boundaries.
			var mergeTimestamps []int64
			if n > 0 && n%2 == 0 {
				for i := n / 4; i < n; i += n / 4 {
					mergeTimestamps = append(mergeTimestamps, baseTime.Add(time.Duration(i)*time.Hour).Unix())
				}
			}

			clusters := Detect(model.ScoredResult{Commits: commits}, mergeTimestamps)

			total := 0
			for _, c := range clusters {
				if c.CommitCount == 0 {
					t.Errorf("n=%d: found a cluster with CommitCount == 0", n)
				}
				total += c.CommitCount
			}
			if total != n {
				t.Errorf("n=%d: sum(cluster.CommitCount) = %d, want %d", n, total, n)
			}
			if len(clusters) > maxClustersPerSync {
				t.Errorf("n=%d: got %d clusters, want <= cap %d", n, len(clusters), maxClustersPerSync)
			}
		})
	}
}

// buildMergeSeparatedCommits produces `groupCount` distinct-category groups of
// 2 commits each, with a merge-commit timestamp between every group so
// groupCommits() sees one boundary per group. Distinct categories keep
// consolidate() from collapsing them for reasons other than the cap.
func buildMergeSeparatedCommits(groupCount int) ([]model.CommitSignal, []int64) {
	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cats := []string{
		"Authentication & Security", "Payments & Billing", "Database & Data Layer",
		"Frontend & UI", "API & Backend Services", "Infrastructure & DevOps",
		"Testing & QA", "Documentation", "Performance & Optimisation",
		"Search & Discovery", "Email & Notifications",
	}
	commits := make([]model.CommitSignal, 0, groupCount*2)
	var mergeTimestamps []int64
	for i := 0; i < groupCount; i++ {
		cat := cats[i%len(cats)]
		groupStart := baseTime.Add(time.Duration(i) * 48 * time.Hour)
		commits = append(commits,
			model.CommitSignal{
				SHA:             "sha" + strconv.Itoa(i) + "a",
				Timestamp:       groupStart,
				Additions:       10,
				PrimaryCategory: cat,
				CategoryScores:  map[string]int{cat: 10},
				ImpactType:      "feature",
			},
			model.CommitSignal{
				SHA:             "sha" + strconv.Itoa(i) + "b",
				Timestamp:       groupStart.Add(time.Hour),
				Additions:       5,
				PrimaryCategory: cat,
				CategoryScores:  map[string]int{cat: 5},
				ImpactType:      "feature",
			},
		)
		mergeTimestamps = append(mergeTimestamps, groupStart.Add(2*time.Hour).Unix())
	}
	return commits, mergeTimestamps
}

// A repo with a healthy PR cadence whose real merge boundaries are at or below
// the cap must keep every one of them — the old code wrongly force-collapsed
// anything past 4.
func TestDetectPreservesRealMergeBoundariesUpToCap(t *testing.T) {
	commits, mergeTimestamps := buildMergeSeparatedCommits(maxClustersPerSync)

	clusters := Detect(model.ScoredResult{Commits: commits}, mergeTimestamps)

	if len(clusters) != maxClustersPerSync {
		t.Fatalf("expected %d merge-derived clusters preserved, got %d", maxClustersPerSync, len(clusters))
	}
	total := 0
	for _, c := range clusters {
		total += c.CommitCount
	}
	if total != len(commits) {
		t.Errorf("sum(cluster.CommitCount) = %d, want %d", total, len(commits))
	}
}

// The headline fix: many more merge boundaries than the cap (a repo with
// dozens of PRs) must consolidate down to exactly the cap — never one
// milestone per PR — while still accounting for every commit.
func TestDetectCapsManyMergeBoundariesAtMax(t *testing.T) {
	const groupCount = 20
	commits, mergeTimestamps := buildMergeSeparatedCommits(groupCount)

	clusters := Detect(model.ScoredResult{Commits: commits}, mergeTimestamps)

	if len(clusters) != maxClustersPerSync {
		t.Fatalf("expected consolidation down to cap %d, got %d clusters", maxClustersPerSync, len(clusters))
	}
	total := 0
	for _, c := range clusters {
		if c.CommitCount == 0 {
			t.Error("consolidated cluster must never be empty")
		}
		total += c.CommitCount
	}
	if total != len(commits) {
		t.Errorf("sum(cluster.CommitCount) = %d, want %d (no commit dropped by consolidation)", total, len(commits))
	}
	// Clusters must stay chronologically ordered and contiguous by index.
	for i, c := range clusters {
		if c.ClusterIndex != i {
			t.Errorf("cluster %d has non-contiguous ClusterIndex %d", i, c.ClusterIndex)
		}
		if i > 0 && c.StartTimestamp < clusters[i-1].StartTimestamp {
			t.Errorf("cluster %d starts before previous cluster — order not preserved", i)
		}
	}
}

// Consolidation must merge adjacent SAME-category groups before it merges any
// cross-category pair, so a feature spread across several PRs collapses into
// one milestone for that feature rather than blending unrelated work.
func TestConsolidatePrefersSameCategoryNeighbours(t *testing.T) {
	mk := func(cat string, n int) []model.CommitSignal {
		g := make([]model.CommitSignal, n)
		for i := range g {
			g[i] = model.CommitSignal{CategoryScores: map[string]int{cat: 5}, PrimaryCategory: cat}
		}
		return g
	}
	// Four groups, three categories: two adjacent Auth groups should merge
	// first, leaving Auth / Payments / Database.
	groups := [][]model.CommitSignal{
		mk("Authentication & Security", 2),
		mk("Authentication & Security", 2),
		mk("Payments & Billing", 2),
		mk("Database & Data Layer", 2),
	}
	out := consolidate(groups, 3)
	if len(out) != 3 {
		t.Fatalf("expected 3 groups after consolidation, got %d", len(out))
	}
	if dominantCategoryOf(out[0]) != "Authentication & Security" || len(out[0]) != 4 {
		t.Errorf("expected first group to be the merged Auth pair (4 commits), got %s (%d)", dominantCategoryOf(out[0]), len(out[0]))
	}
	if dominantCategoryOf(out[1]) != "Payments & Billing" {
		t.Errorf("expected second group Payments, got %s", dominantCategoryOf(out[1]))
	}
	if dominantCategoryOf(out[2]) != "Database & Data Layer" {
		t.Errorf("expected third group Database, got %s", dominantCategoryOf(out[2]))
	}
}

func sizeName(n int) string {
	return "n=" + strconv.Itoa(n)
}

func sha(i int) string {
	return "sha" + strconv.Itoa(i)
}
