package model

import "time"

type State struct {
	LinkedRepos               map[string]LinkedRepoState `json:"linkedRepos"`
	WatchedBranches           []string                   `json:"watchedBranches"`
	AutoUpdateDictionary      bool                       `json:"autoUpdateDictionary"`
	LastDictionaryUpdateCheck time.Time                  `json:"lastDictionaryUpdateCheck,omitempty"`
	SuppressedWorkspaces      []string                   `json:"suppressedWorkspaces"`
	MonthlyCareerSummaryShown map[string]bool            `json:"monthlyCareerSummaryShown"`
	DictionaryVersion         string                     `json:"dictionaryVersion,omitempty"`
	FirstRunSetupComplete     bool                       `json:"firstRunSetupComplete"`
}

type LinkedRepo struct {
	RepoHash string `json:"repoHash"`
	PathHash string `json:"pathHash"`
}

type LinkedRepoState struct {
	RepoHash           string    `json:"repoHash"`
	OrgHash            string    `json:"orgHash"`
	PathHash           string    `json:"pathHash"`
	Provider           string    `json:"provider"`
	LastHeadSHA        string    `json:"lastHeadSha"`
	LastSyncAt         time.Time `json:"lastSyncAt,omitempty"`
	LastHandshake      time.Time `json:"lastHandshake,omitempty"`
	Tier               string    `json:"tier"`
	DictionaryVersion  string    `json:"dictionaryVersion"`
	ProductionBranches []string  `json:"productionBranches"`
}
