package phase2

import (
	"bytes"
	"strings"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

func Classify(commits []model.RawCommit, dictionary model.Dictionary) []model.CommitSignal {
	signals := make([]model.CommitSignal, 0, len(commits))
	for i := range commits {
		commit := &commits[i]
		subjectLower := strings.ToLower(string(commit.Subject))
		scores := make(map[string]int, len(dictionary.Categories))
		primary := CategoryUnknown
		primaryScore := 0
		impact := "feature"

		for category, categorySignals := range dictionary.Categories {
			score := 0
			for _, keyword := range categorySignals.Keywords {
				if strings.Contains(subjectLower, strings.ToLower(keyword)) {
					score++
				}
			}
			for _, filePath := range commit.FilePaths {
				pathLower := strings.ToLower(filePath)
				for _, pattern := range categorySignals.Paths {
					if strings.Contains(pathLower, strings.ToLower(pattern)) {
						score += 2
					}
				}
			}
			if score > 0 {
				scores[category] = score
			}
			if score > primaryScore {
				primary = category
				primaryScore = score
				impact = categorySignals.Impact
			}
		}

		noise := NoiseScore(commit.Subject, scores)
		crypto.ZeroBytes(commit.Subject)
		commit.Subject = nil

		signals = append(signals, model.CommitSignal{
			SHA:             commit.SHA,
			Timestamp:       commit.Timestamp,
			Additions:       commit.Additions,
			Deletions:       commit.Deletions,
			FilesChanged:    commit.FilesChanged,
			AuthorEmailHash: crypto.NormalizedSHA256(commit.AuthorEmail),
			CategoryScores:  scores,
			PrimaryCategory: primary,
			ImpactType:      impact,
			NoiseScore:      noise,
		})
	}
	return signals
}

func NoiseScore(subject []byte, scores map[string]int) float64 {
	trimmed := bytes.TrimSpace(subject)
	if len(trimmed) == 0 {
		return 1
	}
	lower := strings.ToLower(string(trimmed))
	trivial := []string{"wip", "update", "changes", "misc", "fix", "commit"}
	for _, value := range trivial {
		if lower == value {
			return 0.95
		}
	}
	if len(scores) == 0 {
		return 0.75
	}
	return 0.15
}
