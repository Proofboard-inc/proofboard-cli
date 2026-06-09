package model

type Dictionary struct {
	Version    string             `json:"version"`
	Categories map[string]Signals `json:"categories"`
}

type Signals struct {
	Keywords []string `json:"keywords"`
	Paths    []string `json:"paths"`
	Impact   string   `json:"impact"`
}
