package model

type AntiFraudSignals struct {
	AINoiseScore        float64 `json:"aiNoiseScore"`
	OrgHashMismatch     bool    `json:"orgHashMismatch"`
	IdentityMismatch    int     `json:"identityMismatch"`
	LowCommitCount      bool    `json:"lowCommitCount"`
	SingleCommitRepoCap bool    `json:"singleCommitRepoCap"`
	CommitSignatureVerified bool    `json:"commitSignatureVerified"`
	SignedCommitRatio       float64 `json:"signedCommitRatio"`
	CommitIntervalVariance  float64 `json:"commitIntervalVariance"`
	TimeOfDayDistribution   float64 `json:"timeOfDayDistribution"`
	BurstPatternScore       float64 `json:"burstPatternScore"`
}

type SyncPayload struct {
	SHAs              []string         `json:"shas"`
	Timestamps        []int64          `json:"timestamps"`
	Additions         []int            `json:"additions"`
	Deletions         []int            `json:"deletions"`
	FilesChanged      []int            `json:"filesChanged"`
	Categories        []string         `json:"categories"`
	ImpactScores      map[string]float64 `json:"impactScores"`
	MilestoneClusters []Cluster        `json:"milestoneClusters"`
	OrgHash           string           `json:"orgHash"`
	RepoHash          string           `json:"repoHash"`
	EmailHash         string           `json:"emailHash"`
	Provider          string           `json:"provider"`
	AccessStatus      string           `json:"accessStatus"`
	CapturedAt        string           `json:"capturedAt"`
	CLIVersion        string           `json:"cliVersion"`
	DictionaryVersion string           `json:"dictionaryVersion"`
	AntiFraudSignals  AntiFraudSignals `json:"antiFraudSignals"`
}

type SyncReceipt struct {
	ID     string `json:"id"`
	Tier   string `json:"tier"`
	Status string `json:"status"`
}
