package model

type AntiFraudSignals struct {
	AINoiseScore        float64 `json:"aiNoiseScore"`
	OrgHashMismatch     bool    `json:"orgHashMismatch"`
	IdentityMismatch    int     `json:"identityMismatch"`
	LowCommitCount      bool    `json:"lowCommitCount"`
	SingleCommitRepoCap bool    `json:"singleCommitRepoCap"`
	SignedCommitRatio   float64 `json:"signedCommitRatio"`
}

type ImpactScores struct {
	Feature     float64 `json:"feature"`
	Bugfix      float64 `json:"bugfix"`
	Refactor    float64 `json:"refactor"`
	Ship        float64 `json:"ship"`
	Maintenance float64 `json:"maintenance"`
}

type SyncPayload struct {
	SHAs              []string         `json:"shas"`
	Timestamps        []int64          `json:"timestamps"`
	Additions         []int            `json:"additions"`
	Deletions         []int            `json:"deletions"`
	FilesChanged      []int            `json:"filesChanged"`
	Categories        []string         `json:"categories"`
	ImpactScores      ImpactScores     `json:"impactScores"`
	MilestoneClusters []Cluster        `json:"milestoneClusters"`
	OrgHash           string           `json:"orgHash"`
	RepoHash          string           `json:"repoHash"`
	EmailHash         string           `json:"emailHash"`
	Provider          string           `json:"provider"`
	CapturedAt        string           `json:"capturedAt"`
	CLIVersion        string           `json:"cliVersion"`
	DictionaryVersion string           `json:"dictionaryVersion"`
	AntiFraudSignals  AntiFraudSignals `json:"antiFraudSignals"`
	NotifyPush        bool             `json:"notifyPush"`
	PreviousHead      string           `json:"previousHead,omitempty"`
	DeviceKeyID       string           `json:"deviceKeyId,omitempty"`
	DeviceSignature   string           `json:"deviceSignature,omitempty"`
}

type SyncReceipt struct {
	ID      string `json:"id"`
	SyncID  string `json:"syncId,omitempty"`
	Tier    string `json:"tier"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
