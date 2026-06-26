package phase4

import (
	"sort"

	"github.com/proofboard/proofboard/internal/model"
)

func Detect(result model.ScoredResult, mergeTimestamps []int64) []model.Cluster {
	if len(result.Commits) == 0 {
		return nil
	}

	// 1. Sort commits chronologically (ascending)
	commits := make([]model.CommitSignal, len(result.Commits))
	copy(commits, result.Commits)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Timestamp.Before(commits[j].Timestamp)
	})

	// 2. Group commits by cluster index using mergeTimestamps as boundaries.
	// A commit falls into the first merge timestamp M_j >= commit.Timestamp.Unix().
	// If no such M_j exists, it falls into group index len(mergeTimestamps).
	groups := make(map[int][]model.CommitSignal)
	var groupKeys []int

	for _, commit := range commits {
		tUnix := commit.Timestamp.Unix()
		clusterIdx := len(mergeTimestamps)
		for idx, mt := range mergeTimestamps {
			if tUnix <= mt {
				clusterIdx = idx
				break
			}
		}
		if _, ok := groups[clusterIdx]; !ok {
			groupKeys = append(groupKeys, clusterIdx)
		}
		groups[clusterIdx] = append(groups[clusterIdx], commit)
	}

	// Sort group keys to output clusters in chronological order.
	sort.Ints(groupKeys)

	clusters := make([]model.Cluster, 0, len(groupKeys))
	for _, key := range groupKeys {
		clusterCommits := groups[key]
		if len(clusterCommits) == 0 {
			continue
		}

		first := clusterCommits[0].Timestamp
		last := clusterCommits[len(clusterCommits)-1].Timestamp
		additions := 0
		deletions := 0
		filesChanged := 0
		impactScores := make(map[string]int)
		categorySums := make(map[string]int)

		for _, commit := range clusterCommits {
			additions += commit.Additions
			deletions += commit.Deletions
			filesChanged += commit.FilesChanged
			impactScores[commit.ImpactType]++
			for cat, score := range commit.CategoryScores {
				categorySums[cat] += score
			}
		}

		type rankedCategory struct {
			name  string
			score int
		}
		var ranked []rankedCategory
		for cat, score := range categorySums {
			ranked = append(ranked, rankedCategory{name: cat, score: score})
		}
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].score == ranked[j].score {
				return ranked[i].name < ranked[j].name
			}
			return ranked[i].score > ranked[j].score
		})

		primary := "Unclassified"
		if len(ranked) > 0 {
			primary = ranked[0].name
		}

		scaleStr := scale(len(clusterCommits))
		impType := dominantImpact(impactScores)

		c := model.Cluster{
			Category:          primary,
			ImpactType:        impType,
			ImpactScale:       scaleStr,
			CommitCount:       len(clusterCommits),
			TotalAdditions:    additions,
			TotalDeletions:    deletions,
			TotalFilesChanged: filesChanged,
			StartTimestamp:    first.Unix(),
			EndTimestamp:      last.Unix(),
			ClusterIndex:      key,
		}

		clusters = append(clusters, c)
	}

	return clusters
}

func scale(commitCount int) string {
	switch {
	case commitCount > 30:
		return "large"
	case commitCount >= 10:
		return "medium"
	default:
		return "small"
	}
}

func dominantImpact(scores map[string]int) string {
	best := "feature"
	bestScore := -1
	for impact, score := range scores {
		if score > bestScore || (score == bestScore && impact < best) {
			best = impact
			bestScore = score
		}
	}
	return best
}

func representativeSHAs(commits []model.CommitSignal) []string {
	limit := 3
	if len(commits) < limit {
		limit = len(commits)
	}
	shas := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		shas = append(shas, commits[i].SHA)
	}
	return shas
}
