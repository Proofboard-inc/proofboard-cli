package model

import "time"

type SyncPayload struct {
	SHAs              []string       `json:"shas"`
	Timestamps        []int64        `json:"timestamps"`
	Additions         []int          `json:"additions"`
	Deletions         []int          `json:"deletions"`
	FilesChanged      []int          `json:"filesChanged"`
	Categories        []string       `json:"categories"`
	ImpactScores      map[string]int `json:"impactScores"`
	MilestoneClusters []Cluster      `json:"milestoneClusters"`
	OrgHash           string         `json:"orgHash"`
	RepoHash          string         `json:"repoHash"`
	EmailHash         string         `json:"emailHash"`
	HandshakeStatus   string         `json:"handshakeStatus"`
	CapturedAt        time.Time      `json:"capturedAt"`
	CLIVersion        string         `json:"cliVersion"`
	DictionaryVersion string         `json:"dictionaryVersion"`
	OrgHashMismatch   bool           `json:"orgHashMismatch"`
	IdentityMismatch  int            `json:"identityMismatch"`
	LowCommitCount    bool           `json:"lowCommitCount"`
	AINoiseScore      float64        `json:"aiNoiseScore"`
}

type SyncReceipt struct {
	ID     string `json:"id"`
	Tier   string `json:"tier"`
	Status string `json:"status"`
}
