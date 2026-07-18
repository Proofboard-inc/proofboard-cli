package model

import "time"

type Credentials struct {
	Token        string    `json:"token"`
	Username     string    `json:"username"`
	RefreshToken string    `json:"refreshToken"`
	EmailHash    string    `json:"emailHash"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}
