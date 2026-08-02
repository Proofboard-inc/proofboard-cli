package model

type Cluster struct {
	Category          string `json:"category"`
	ImpactType        string `json:"impactType"`
	ImpactScale       string `json:"impactScale"`
	CommitCount       int    `json:"commitCount"`
	TotalAdditions    int    `json:"totalAdditions"`
	TotalDeletions    int    `json:"totalDeletions"`
	StartTimestamp    int64  `json:"startTimestamp"`
	EndTimestamp      int64  `json:"endTimestamp"`
	TotalFilesChanged int    `json:"totalFilesChanged"`
	ClusterIndex      int    `json:"clusterIndex"`
	// ReferenceShas are a small, evenly-spaced sample of this cluster's OWN
	// commit SHAs (first/middle/last, up to 3) — real commits that actually
	// belong to this milestone. Only SHA hashes leave the machine, same as
	// the top-level shas[] list; no messages/paths/diffs. Lets the backend
	// show real reference commits per milestone instead of an arbitrary
	// index-based slice of the unrelated flat shas[] array.
	ReferenceShas []string `json:"referenceShas,omitempty"`
	// FeatureKeyword is a generic, resume-style feature noun ("dashboard",
	// "checkout") — a separate, more specific signal from Category, matched
	// against Dictionary.FeatureKeywords. Gives the AI outcome-summary prompt
	// something concrete to write about instead of two vague category labels.
	// Empty if no commit in this cluster matched any feature keyword.
	FeatureKeyword string `json:"featureKeyword,omitempty"`
}

type ScoredResult struct {
	Commits           []CommitSignal
	CategoryTotals    map[string]int
	ContributionAreas []string
	PrimaryCategory   string
	DictionaryVersion string
}
