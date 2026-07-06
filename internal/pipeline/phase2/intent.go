package phase2

import (
	"bytes"
	"strings"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

func toLowerBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		} else {
			out[i] = c
		}
	}
	return out
}

func Classify(commits []model.RawCommit, dictionary model.Dictionary) []model.CommitSignal {
	signals := make([]model.CommitSignal, 0, len(commits))
	for i := range commits {
		commit := &commits[i]
		subjectLowerBytes := toLowerBytes(commit.Subject)
		scores := make(map[string]int, len(dictionary.Categories))
		primary := CategoryUnknown
		primaryScore := 0
		impact := "feature"

		for category, categorySignals := range dictionary.Categories {
			score := 0
			for _, keyword := range categorySignals.Keywords {
				keywordBytes := []byte(strings.ToLower(keyword))
				if bytes.Contains(subjectLowerBytes, keywordBytes) {
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
		crypto.ZeroBytes(subjectLowerBytes)
		subjectLowerBytes = nil

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
			SignatureValid:  commit.SignatureValid,
		})
	}
	return signals
}

func NoiseScore(subject []byte, scores map[string]int) float64 {
	trimmed := bytes.TrimSpace(subject)
	if len(trimmed) == 0 {
		return 1
	}
	trimmedLower := toLowerBytes(trimmed)
	defer func() {
		crypto.ZeroBytes(trimmedLower)
	}()
	trivial := [][]byte{
		[]byte("wip"),
		[]byte("update"),
		[]byte("changes"),
		[]byte("misc"),
		[]byte("fix"),
		[]byte("commit"),
	}
	for _, value := range trivial {
		if bytes.Equal(trimmedLower, value) {
			return 0.95
		}
	}
	if len(scores) == 0 {
		return 0.75
	}
	return 0.15
}
