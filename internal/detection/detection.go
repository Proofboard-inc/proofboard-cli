package detection

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/proofboard/proofboard/internal/git"
	statestore "github.com/proofboard/proofboard/internal/state"
)

type Action string

const (
	ActionNone       Action = "none"
	ActionLink       Action = "link"
	ActionSync       Action = "sync"
	ActionSuppressed Action = "suppressed"
)

type Result struct {
	Action          Action `json:"action"`
	Editor          string `json:"editor,omitempty"`
	WorkspacePath   string `json:"workspacePath"`
	RepoPath        string `json:"repoPath,omitempty"`
	RepoName        string `json:"repoName,omitempty"`
	RepoHash        string `json:"repoHash,omitempty"`
	OrgHash         string `json:"orgHash,omitempty"`
	Provider        string `json:"provider,omitempty"`
	LastHeadSHA     string `json:"lastHeadSha,omitempty"`
	CurrentHeadSHA  string `json:"currentHeadSha,omitempty"`
	SuggestedAction string `json:"suggestedAction,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Suppressed      bool   `json:"suppressed"`
	AlreadyPrompted bool   `json:"alreadyPrompted"`
	Linked          bool   `json:"linked"`
	OutOfDate       bool   `json:"outOfDate"`
	MetadataChanged bool   `json:"metadataChanged"`
}

func Inspect(ctx context.Context, homeDir, workspacePath, editor string) (Result, error) {
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve workspace path: %w", err)
	}

	repo, err := git.Discover(ctx, absWorkspace)
	if err != nil {
		return Result{}, err
	}
	remoteURL, err := git.OriginURL(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	identity, err := git.ParseRemote(remoteURL)
	if err != nil {
		return Result{}, err
	}

	store := statestore.NewStore(homeDir)
	stateData, err := store.Load(ctx)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Editor:        editor,
		WorkspacePath: absWorkspace,
		RepoPath:      repo.Path,
		RepoName:      identity.Repo,
		RepoHash:      identity.RepoHash,
		OrgHash:       identity.OrgHash,
		Provider:      identity.Provider,
	}

	if statestore.IsWorkspaceSuppressed(stateData, absWorkspace) {
		result.Action = ActionSuppressed
		result.Suppressed = true
		result.Reason = "workspace suppressed"
		return result, nil
	}

	repoState, linked := stateData.LinkedRepos[identity.RepoHash]
	result.Linked = linked
	if !linked {
		// The connection prompt is offered once per workspace. Editors open a
		// new shell for every terminal tab, so prompting on each inspection
		// would re-ask the same question all day.
		if statestore.WasWorkspacePrompted(stateData, absWorkspace) {
			result.Action = ActionNone
			result.AlreadyPrompted = true
			result.Reason = "connection prompt already offered for this workspace"
			return result, nil
		}
		result.Action = ActionLink
		result.SuggestedAction = "Sync Project"
		result.Reason = "project is not connected"
		return result, nil
	}

	currentHead, err := git.Head(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	result.LastHeadSHA = repoState.LastHeadSHA
	result.CurrentHeadSHA = currentHead
	metadataHash, err := git.MetadataFingerprint(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	result.MetadataChanged = repoState.MetadataHash == "" || repoState.MetadataHash != metadataHash

	if repoState.LastHeadSHA == "" || !strings.EqualFold(repoState.LastHeadSHA, currentHead) || result.MetadataChanged {
		result.Action = ActionSync
		result.OutOfDate = true
		result.SuggestedAction = "Sync Project"
		if result.MetadataChanged && strings.EqualFold(repoState.LastHeadSHA, currentHead) {
			result.Reason = "repository metadata changed since the last sync"
		} else {
			result.Reason = "workspace has new commits since the last sync"
		}
		return result, nil
	}

	result.Action = ActionNone
	result.Reason = "project already connected and synchronized"
	return result, nil
}

func (r Result) Marshal() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func (r Result) HumanMessage() string {
	switch r.Action {
	case ActionLink:
		return fmt.Sprintf("Proofboard Career Agent: New repository detected\n%s\nChoose Sync Project to begin private local tracking.", r.RepoName)
	case ActionSync:
		return fmt.Sprintf("Proofboard Career Agent: Syncing %s in the background.", r.RepoName)
	case ActionSuppressed:
		return ""
	default:
		return ""
	}
}
