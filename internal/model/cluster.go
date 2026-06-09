package model

type Cluster struct {
	ClusterLabel       string   `json:"clusterLabel"`
	ImpactType         string   `json:"impactType"`
	Scale              string   `json:"scale"`
	CommitCount        int      `json:"commitCount"`
	AdditionTotal      int      `json:"additionTotal"`
	DeletionTotal      int      `json:"deletionTotal"`
	DurationDays       int      `json:"durationDays"`
	ReferenceSHABucket []string `json:"referenceShaBucket"`
}

type ScoredResult struct {
	Commits           []CommitSignal
	CategoryTotals    map[string]int
	ContributionAreas []string
	PrimaryCategory   string
	DictionaryVersion string
}
