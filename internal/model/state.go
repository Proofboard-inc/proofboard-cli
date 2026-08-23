package model

import "time"

type State struct {
	LinkedRepos               map[string]LinkedRepoState `json:"linkedRepos"`
	WatchedBranches           []string                   `json:"watchedBranches"`
	AutoUpdateDictionary      bool                       `json:"autoUpdateDictionary"`
	LastDictionaryUpdateCheck time.Time                  `json:"lastDictionaryUpdateCheck,omitempty"`
	SuppressedWorkspaces      []string                   `json:"suppressedWorkspaces"`
	PromptedWorkspaces        map[string]time.Time       `json:"promptedWorkspaces,omitempty"`
	DictionaryVersion         string                     `json:"dictionaryVersion,omitempty"`
	FirstRunSetupComplete     bool                       `json:"firstRunSetupComplete"`
	IDEProcesses              []string                   `json:"ideProcesses,omitempty"`
	AuthReconnectPrompted     bool                       `json:"authReconnectPrompted,omitempty"`
	AuthReconnectPromptedAt   time.Time                  `json:"authReconnectPromptedAt,omitempty"`
	AuthLoggedOut             bool                       `json:"authLoggedOut,omitempty"`
	// KeychainDisabled forces device-signing-key storage to the plaintext
	// ~/.proofboard/device.key file instead of the OS keychain. Zero-value
	// (false) keeps the OS keychain as the default; this is an explicit
	// opt-out for environments where OS keychain access isn't reachable
	// (e.g. some non-standard terminal/session contexts), set via
	// `proofboard config set keychain-disabled true`.
	KeychainDisabled bool `json:"keychainDisabled,omitempty"`
	// AutoUpdateCLIDisabled opts this machine out of the daily background
	// executable update. The field is negative on purpose: its zero value
	// leaves auto-update ON, so an existing state.json written before this
	// field existed keeps the intended default rather than silently opting
	// out every install that upgrades into it.
	AutoUpdateCLIDisabled bool `json:"autoUpdateCliDisabled,omitempty"`
	// LastVersionCheck throttles the "a new version is available" notice.
	// It used to run on every single command, so every invocation made a
	// network call, and because it shared one deadline with the dictionary
	// check that followed it, a slow response left the dictionary with an
	// already-expired context and it failed every time.
	LastVersionCheck time.Time `json:"lastVersionCheck,omitempty"`
	// LastCLIUpdateCheck throttles that check to once a day. Separate from
	// LastDictionaryUpdateCheck because the two run on different cadences and
	// a failure in one must not defer the other.
	LastCLIUpdateCheck time.Time `json:"lastCliUpdateCheck,omitempty"`
	// RecoveredLegacyPrompts marks that the one-time recovery from the
	// legacy-backgrounded-shell-hook bug has already run (see
	// state.RecoverBurnedWorkspacePrompts). Prevents that recovery from
	// re-clearing PromptedWorkspaces on every subsequent hook migration.
	RecoveredLegacyPrompts bool `json:"recoveredLegacyPrompts,omitempty"`
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
	// LastSyncPayload caches the exact payload most recently transmitted for
	// this repo (post-Shredder, contains no commit text, same content that
	// already left the machine). Replayed verbatim by `sync --resync` so the
	// resend's contentHash matches what the backend already has on file,
	// without re-ingesting/re-classifying git history. Nil until the first
	// successful sync.
	LastSyncPayload *SyncPayload `json:"lastSyncPayload,omitempty"`
}
