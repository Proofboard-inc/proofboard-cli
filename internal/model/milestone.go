package model

type MilestoneBundle struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	OutcomeSummary string `json:"outcomeSummary,omitempty"`
	Category       string `json:"category,omitempty"`
	Status         string `json:"status,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
}

type MilestoneBundlePage struct {
	Data []MilestoneBundle `json:"data"`
}
