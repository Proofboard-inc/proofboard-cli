package model

import "time"

type RawCommit struct {
	SHA          string
	Timestamp    time.Time
	Additions    int
	Deletions    int
	FilesChanged int
	Subject      []byte
	// Full commit message body (everything after the subject line).
	// Fetched via a separate `git log %b` call (see internal/git/log.go) to
	// avoid smuggling multi-line free text through the single-line-oriented
	// header/numstat parser. Classified alongside Subject in phase2, then
	// zeroed and nil'd the same way; never survives past classification.
	Body           []byte
	FilePaths      []string
	AuthorEmail    string
	Repository     string
	Organization   string
	SignatureValid bool
}

type CommitSignal struct {
	SHA             string
	Timestamp       time.Time
	Additions       int
	Deletions       int
	FilesChanged    int
	AuthorEmailHash string
	CategoryScores  map[string]int
	PrimaryCategory string
	// FeatureKeyword is the best-matching generic feature noun for this
	// commit (e.g. "dashboard", "checkout"), matched against
	// Dictionary.FeatureKeywords, a separate, more specific signal from the
	// 25-category taxonomy. Empty if nothing matched. Kept alongside
	// FeatureKeywords (plural) for callers that only need the single
	// strongest signal.
	FeatureKeyword string
	// FeatureKeywords holds EVERY dictionary keyword that matched this commit
	// (subject/body/path), not just the top-scoring one: a commit touching
	// both an "orders" and "delivery" area should contribute to both counts
	// when a cluster later tallies its dominant feature keywords, instead of
	// only ever counting whichever one happened to score highest. Ordered by
	// descending match score.
	FeatureKeywords []string
	ImpactType      string
	NoiseScore      float64
	SignatureValid  bool
}

type SafeCommit struct {
	SHA             string
	TimestampUnix   int64
	Additions       int
	Deletions       int
	FilesChanged    int
	Category        string
	ImpactType      string
	NoiseScore      float64
	AuthorEmailHash string
	SignatureValid  bool
}
