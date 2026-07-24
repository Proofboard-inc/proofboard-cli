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

	clusterGroups := groupCommits(commits, mergeTimestamps)
	if len(clusterGroups) == 1 && len(commits) >= 5 {
		clusterGroups = splitCommits(commits, 4)
	} else if len(clusterGroups) > 4 {
		clusterGroups = splitCommits(commits, 4)
	}

	clusters := make([]model.Cluster, 0, len(clusterGroups))
	for idx, clusterCommits := range clusterGroups {
		if len(clusterCommits) == 0 {
			continue
		}
		clusters = append(clusters, buildCluster(clusterCommits, idx))
	}

	return clusters
}

func groupCommits(commits []model.CommitSignal, mergeTimestamps []int64) [][]model.CommitSignal {
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

	sort.Ints(groupKeys)
	ordered := make([][]model.CommitSignal, 0, len(groupKeys))
	for _, key := range groupKeys {
		ordered = append(ordered, groups[key])
	}
	return ordered
}

func splitCommits(commits []model.CommitSignal, maxClusters int) [][]model.CommitSignal {
	if len(commits) == 0 {
		return nil
	}
	parts := len(commits)
	if parts > maxClusters {
		parts = maxClusters
	}
	base := len(commits) / parts
	extra := len(commits) % parts
	groups := make([][]model.CommitSignal, 0, parts)
	offset := 0
	for i := 0; i < parts; i++ {
		size := base
		if i < extra {
			size++
		}
		next := offset + size
		groups = append(groups, commits[offset:next])
		offset = next
	}
	return groups
}

func buildCluster(clusterCommits []model.CommitSignal, clusterIndex int) model.Cluster {
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

	primary := "Feature Development"
	if len(ranked) > 0 {
		primary = ranked[0].name
	}
	if primary == "Unclassified" {
		primary = "Feature Development"
	}

	return model.Cluster{
		Category:          primary,
		ImpactType:        dominantImpact(impactScores),
		ImpactScale:       scale(len(clusterCommits)),
		CommitCount:       len(clusterCommits),
		TotalAdditions:    additions,
		TotalDeletions:    deletions,
		TotalFilesChanged: filesChanged,
		StartTimestamp:    first.Unix(),
		EndTimestamp:      last.Unix(),
		ClusterIndex:      clusterIndex,
	}
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
