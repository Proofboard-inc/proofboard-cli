package model

import "time"

type Notification struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	IsRead    bool           `json:"isRead"`
	ActionURL string         `json:"actionUrl"`
	Meta      map[string]any `json:"meta"`
	CreatedAt time.Time      `json:"createdAt"`
}

type PaginatedNotifications struct {
	Data []Notification `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

type PaginationMeta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"totalPages"`
}

type UnreadCountResponse struct {
	Count int `json:"count"`
}
