package model

import "time"

type State struct {
	LinkedRepos               map[string]LinkedRepoState `json:"linkedRepos"`
	WatchedBranches           []string                   `json:"watchedBranches"`
	AutoUpdateDictionary      bool                       `json:"autoUpdateDictionary"`
	LastDictionaryUpdateCheck time.Time                  `json:"lastDictionaryUpdateCheck,omitempty"`
	SuppressedWorkspaces      []string                   `json:"suppressedWorkspaces"`
	PromptedWorkspaces        map[string]time.Time       `json:"promptedWorkspaces,omitempty"`
	MonthlyCareerSummaryShown map[string]bool            `json:"monthlyCareerSummaryShown"`
	DictionaryVersion         string                     `json:"dictionaryVersion,omitempty"`
	FirstRunSetupComplete     bool                       `json:"firstRunSetupComplete"`
	IDEProcesses              []string                   `json:"ideProcesses,omitempty"`
	AuthReconnectPrompted     bool                       `json:"authReconnectPrompted,omitempty"`
	AuthReconnectPromptedAt   time.Time                  `json:"authReconnectPromptedAt,omitempty"`
	AuthLoggedOut             bool                       `json:"authLoggedOut,omitempty"`
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
	ProjectID          string    `json:"projectId"`
	PublicKey          string    `json:"publicKey"`
	EmailHashKey       string    `json:"emailHashKey,omitempty"`
	DictionaryVersion  string    `json:"dictionaryVersion"`
	ProductionBranches []string  `json:"productionBranches"`
	MetadataHash       string    `json:"metadataHash,omitempty"`
}
