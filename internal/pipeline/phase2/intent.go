package phase2

import (
	"bytes"
	"sort"
	"strings"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

// sortedCategoryNames returns the dictionary's category names in a stable,
// deterministic order. Go randomizes map iteration order per process, which
// previously made Classify's tie-break (first category to reach the top
// score wins, via strict `>`) non-deterministic whenever two categories tied
// on score for the same commit. Iterating this sorted slice instead of
// ranging the map directly fixes that: ties now always resolve to the
// alphabetically-first tied category, consistent with the deterministic
// tie-breaks already used in phase3 (scoring.go) and phase4 (milestones.go).
func sortedCategoryNames(categories map[string]model.Signals) []string {
	names := make([]string, 0, len(categories))
	for name := range categories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

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

// Classification signal weights, ordered by how much they can be trusted:
//
//   - pathWeight (strongest): a file-path match is structural — it reflects
//     which files the commit actually changed, which the author can't fake in
//     a message. It dominates, and also breaks score ties (see below).
//   - subjectKeywordWeight: the subject line is the author's intentional
//     one-line summary — a strong but text-based (spoofable) signal.
//   - bodyKeywordWeight / symbolWeight (recall): the body and symbol-like
//     identifiers add coverage when the subject is uninformative, but are
//     noisier, so they count least.
const (
	pathWeight           = 3
	subjectKeywordWeight = 2
	bodyKeywordWeight    = 1
	symbolWeight         = 1
)

func Classify(commits []model.RawCommit, dictionary model.Dictionary) []model.CommitSignal {
	signals := make([]model.CommitSignal, 0, len(commits))
	for i := range commits {
		commit := &commits[i]
		subjectLowerBytes := toLowerBytes(commit.Subject)
		// Body is a big accuracy lever — a subject like "updates" tells
		// the classifier nothing, but the fuller body text often does. Weighted
		// below the subject (subjects are the author's intentional summary,
		// bodies add recall).
		bodyLowerBytes := toLowerBytes(commit.Body)
		symbols := ExtractSymbols(bodyLowerBytes, commit.FilePaths)
		scores := make(map[string]int, len(dictionary.Categories))
		pathHit := make(map[string]bool, len(dictionary.Categories))
		primary := CategoryUnknown
		primaryScore := 0
		primaryPath := false
		impact := "feature"

		for _, category := range sortedCategoryNames(dictionary.Categories) {
			categorySignals := dictionary.Categories[category]
			score := 0
			for _, keyword := range categorySignals.Keywords {
				keywordBytes := []byte(strings.ToLower(keyword))
				if bytes.Contains(subjectLowerBytes, keywordBytes) {
					score += subjectKeywordWeight
				}
				if bodyLowerBytes != nil && bytes.Contains(bodyLowerBytes, keywordBytes) {
					score += bodyKeywordWeight
				}
				keywordLower := string(keywordBytes)
				for _, symbol := range symbols {
					if strings.Contains(symbol, keywordLower) {
						score += symbolWeight
					}
				}
			}
			hasPath := false
			for _, filePath := range commit.FilePaths {
				pathLower := strings.ToLower(filePath)
				for _, pattern := range categorySignals.Paths {
					if strings.Contains(pathLower, strings.ToLower(pattern)) {
						score += pathWeight
						hasPath = true
					}
				}
			}
			if score > 0 {
				scores[category] = score
			}
			if hasPath {
				pathHit[category] = true
			}
			// Higher score wins. On a tie, the category backed by a structural
			// file-path match beats a purely text-based one; only if that is
			// also equal does the alphabetically-first name win (categories are
			// iterated in sorted order, so the first qualifying one sticks).
			better := score > primaryScore ||
				(score == primaryScore && score > 0 && hasPath && !primaryPath)
			if better {
				primary = category
				primaryScore = score
				primaryPath = hasPath
				impact = categorySignals.Impact
			}
		}

		// FeatureKeyword: a separate, more specific signal than category — see
		// model.Dictionary.FeatureKeywords. Iterated in dictionary list order
		// (not a map), so a tie between two keywords deterministically keeps
		// whichever is listed first — no extra sort needed, unlike the
		// category loop above.
		featureKeyword := ""
		featureScore := 0
		for _, keyword := range dictionary.FeatureKeywords {
			keywordBytes := []byte(strings.ToLower(keyword))
			score := 0
			if bytes.Contains(subjectLowerBytes, keywordBytes) {
				score += subjectKeywordWeight
			}
			if bodyLowerBytes != nil && bytes.Contains(bodyLowerBytes, keywordBytes) {
				score += bodyKeywordWeight
			}
			if score > featureScore {
				featureScore = score
				featureKeyword = keyword
			}
		}

		noise := NoiseScore(commit.Subject, scores)
		crypto.ZeroBytes(commit.Subject)
		commit.Subject = nil
		crypto.ZeroBytes(commit.Body)
		commit.Body = nil
		crypto.ZeroBytes(bodyLowerBytes)
		bodyLowerBytes = nil
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
			FeatureKeyword:  featureKeyword,
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
