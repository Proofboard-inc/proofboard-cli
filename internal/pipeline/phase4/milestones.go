package phase4

import "github.com/proofboard/proofboard/internal/model"

func Detect(result model.ScoredResult) []model.Cluster {
	if len(result.Commits) == 0 {
		return nil
	}
	first := result.Commits[0].Timestamp
	last := first
	additions := 0
	deletions := 0
	impactScores := make(map[string]int)
	for _, commit := range result.Commits {
		if commit.Timestamp.Before(first) {
			first = commit.Timestamp
		}
		if commit.Timestamp.After(last) {
			last = commit.Timestamp
		}
		additions += commit.Additions
		deletions += commit.Deletions
		impactScores[commit.ImpactType]++
	}
	return []model.Cluster{{
		ClusterLabel:       result.PrimaryCategory,
		ImpactType:         dominantImpact(impactScores),
		Scale:              scale(len(result.Commits)),
		CommitCount:        len(result.Commits),
		AdditionTotal:      additions,
		DeletionTotal:      deletions,
		DurationDays:       int(last.Sub(first).Hours() / 24),
		ReferenceSHABucket: representativeSHAs(result.Commits),
	}}
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
		if score > bestScore {
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
