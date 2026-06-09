package phase3

import (
	"sort"

	"github.com/proofboard/proofboard/internal/model"
)

func Score(commits []model.CommitSignal, dictionaryVersion string) model.ScoredResult {
	totals := make(map[string]int)
	for _, commit := range commits {
		for category, score := range commit.CategoryScores {
			totals[category] += score
		}
	}
	type rankedCategory struct {
		name  string
		score int
	}
	ranked := make([]rankedCategory, 0, len(totals))
	for category, score := range totals {
		if score >= 3 {
			ranked = append(ranked, rankedCategory{name: category, score: score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].name < ranked[j].name
		}
		return ranked[i].score > ranked[j].score
	})
	limit := 4
	if len(ranked) < limit {
		limit = len(ranked)
	}
	areas := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		areas = append(areas, ranked[i].name)
	}
	primary := ""
	if len(areas) > 0 {
		primary = areas[0]
	}
	return model.ScoredResult{
		Commits:           commits,
		CategoryTotals:    totals,
		ContributionAreas: areas,
		PrimaryCategory:   primary,
		DictionaryVersion: dictionaryVersion,
	}
}
