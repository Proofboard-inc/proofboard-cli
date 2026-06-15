package model

import "time"

type ActivityLog struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	CreatedAt time.Time      `json:"createdAt"`
	Meta      map[string]any `json:"meta"`
}

type PaginatedActivityLogs struct {
	Data []ActivityLog  `json:"data"`
	Meta PaginationMeta `json:"meta"`
}
