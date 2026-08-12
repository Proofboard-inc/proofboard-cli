package phase2

import (
	"bytes"
	"regexp"
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

// conventionalCommitPrefix matches a Conventional Commits type prefix at the
// start of a subject line: "feat:", "fix(scope):", "chore!:", etc.
var conventionalCommitPrefix = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?!?:\s`)

// conventionalCommitImpact maps a Conventional Commits type to an impact
// type. This is a STRONGER signal than the category dictionary's default
// impact for a matched category: the author explicitly declared what kind of
// change this is, so it overrides the category-inferred guess rather than
// the other way around — previously a "chore: bump dependency versions"
// commit that happened to match e.g. the "API & Backend Services" category
// by keyword coincidence would be classified impact "feature" (that
// category's default), which is wrong regardless of what the category is.
var conventionalCommitImpact = map[string]string{
	"feat":     "feature",
	"feature":  "feature",
	"fix":      "bugfix",
	"bugfix":   "bugfix",
	"hotfix":   "bugfix",
	"revert":   "bugfix",
	"refactor": "refactor",
	"perf":     "refactor",
	"chore":    "maintenance",
	"style":    "maintenance",
	"docs":     "maintenance",
	"test":     "maintenance",
	"tests":    "maintenance",
	"build":    "ship",
	"ci":       "ship",
	"deploy":   "ship",
}

// impactFromConventionalPrefix returns the impact type declared by a
// Conventional Commits prefix on the subject line, or "" if the subject
// doesn't have one or the type isn't recognized.
func impactFromConventionalPrefix(subject []byte) string {
	match := conventionalCommitPrefix.FindSubmatch(subject)
	if match == nil {
		return ""
	}
	return conventionalCommitImpact[strings.ToLower(string(match[1]))]
}

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

		// A Conventional Commits type prefix ("chore:", "fix:", ...) is an
		// explicit, author-declared signal — it overrides whatever impact the
		// matched category defaults to (see impactFromConventionalPrefix).
		if declared := impactFromConventionalPrefix(subjectLowerBytes); declared != "" {
			impact = declared
		}

		// FeatureKeyword(s): a separate, more specific signal than category —
		// see model.Dictionary.FeatureKeywords. Every keyword that scores > 0
		// is kept (not just the single top-scorer) — a commit can legitimately
		// touch more than one concept (e.g. an "orders" commit that also wires
		// up "delivery"), and dropping every match but the best one was
		// throwing that signal away, leaving milestone clusters with only one
		// generic feature label even when several distinct ones were present
		// across their commits. Sorted by descending score, then by dictionary
		// order for a deterministic tie-break (not a map, so no extra sort key
		// needed there).
		//
		// File-path matching (weighted highest, same as category classification
		// above) was added after real-world dogfooding showed most commit
		// subjects are too terse ("fix bug", "update logic") to ever contain a
		// feature phrase — but the module/folder a commit actually touches
		// (src/modules/vendors/..., src/modules/delivery/...) reliably names the
		// feature area regardless of how the author wrote the message, and can't
		// be spoofed by a vague commit message the way subject/body text can.
		type featureMatch struct {
			keyword string
			score   int
		}
		var featureMatches []featureMatch
		for _, keyword := range dictionary.FeatureKeywords {
			keywordBytes := []byte(strings.ToLower(keyword))
			score := 0
			if bytes.Contains(subjectLowerBytes, keywordBytes) {
				score += subjectKeywordWeight
			}
			if bodyLowerBytes != nil && bytes.Contains(bodyLowerBytes, keywordBytes) {
				score += bodyKeywordWeight
			}
			for _, filePath := range commit.FilePaths {
				if strings.Contains(strings.ToLower(filePath), string(keywordBytes)) {
					score += pathWeight
					break
				}
			}
			if score > 0 {
				featureMatches = append(featureMatches, featureMatch{keyword, score})
			}
		}
		sort.SliceStable(featureMatches, func(i, j int) bool {
			return featureMatches[i].score > featureMatches[j].score
		})
		featureKeyword := ""
		featureKeywords := make([]string, 0, len(featureMatches))
		for _, m := range featureMatches {
			featureKeywords = append(featureKeywords, m.keyword)
		}
		if len(featureKeywords) > 0 {
			featureKeyword = featureKeywords[0]
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
			FeatureKeywords: featureKeywords,
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
