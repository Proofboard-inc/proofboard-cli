package model

type AntiFraudSignals struct {
	AINoiseScore        float64 `json:"aiNoiseScore"`
	OrgHashMismatch     bool    `json:"orgHashMismatch"`
	IdentityMismatch    int     `json:"identityMismatch"`
	LowCommitCount      bool    `json:"lowCommitCount"`
	SingleCommitRepoCap bool    `json:"singleCommitRepoCap"`
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
	HandshakeStatus   string           `json:"handshakeStatus"`
	CapturedAt        string           `json:"capturedAt"`
	CLIVersion        string           `json:"cliVersion"`
	DictionaryVersion string           `json:"dictionaryVersion"`
	AntiFraudSignals  AntiFraudSignals `json:"antiFraudSignals"`
	NotifyPush        bool             `json:"notifyPush"`
}

type SyncReceipt struct {
	ID     string `json:"id"`
	Tier   string `json:"tier"`
	Status string `json:"status"`
}
