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
}

type ScoredResult struct {
	Commits           []CommitSignal
	CategoryTotals    map[string]int
	ContributionAreas []string
	PrimaryCategory   string
	DictionaryVersion string
}
