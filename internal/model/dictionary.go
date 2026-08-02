package model

type Dictionary struct {
	Version    string             `json:"version"`
	Categories map[string]Signals `json:"categories"`
	// FeatureKeywords is a separate, flat vocabulary of generic, resume-style
	// feature nouns ("dashboard", "checkout", "onboarding") — NOT another
	// tier of the 25-category taxonomy. A commit's PrimaryCategory answers
	// "what kind of work" (Frontend & UI, API & Backend Services, ...); a
	// FeatureKeyword answers "which feature" (dashboard, checkout), giving
	// the AI outcome-summary prompt something concrete to write about instead
	// of two categories that are both still vague. Every entry is a generic
	// term common across SaaS/web/mobile products — never anything specific
	// to one company or codebase.
	FeatureKeywords []string `json:"featureKeywords,omitempty"`
}

type Signals struct {
	Keywords []string `json:"keywords"`
	Paths    []string `json:"paths"`
	Impact   string   `json:"impact"`
}
