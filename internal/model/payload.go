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
	// Locally-detected tech stack + structural signals, refreshed on
	// every sync. Optional — old CLI versions simply omit it.
	Stack *StackReport `json:"stack,omitempty"`
	// IsDefaultBranch reports whether the branch this sync's commits came
	// from is the repository's actual detected default branch (per
	// detectDefaultBranch), as opposed to one manually added via
	// `proofboard config add-branch`. Never the branch name itself — just
	// this boolean — so nothing proprietary/identifying leaves the machine.
	// Lets the backend weight SHA-proof trust higher for verifiably-shipped
	// (default-branch) work than for manually-vouched-for branch work, which
	// the CLI itself has no way to verify is actually a legitimate long-lived
	// branch rather than a throwaway one a user pointed the tool at.
	IsDefaultBranch bool `json:"isDefaultBranch"`
	// Regenerate, when true, tells the backend this is a `sync --resync`
	// replay of an already-ingested payload: no new commits, just a request
	// to regenerate milestone-cluster outcomeSummary text. Omitted (false)
	// on every normal sync.
	Regenerate bool `json:"regenerate,omitempty"`
}

type SyncReceipt struct {
	ID      string `json:"id"`
	SyncID  string `json:"syncId,omitempty"`
	Tier    string `json:"tier"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
