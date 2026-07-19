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
	Action           Action `json:"action"`
	Editor           string `json:"editor,omitempty"`
	WorkspacePath    string `json:"workspacePath"`
	RepoPath         string `json:"repoPath,omitempty"`
	RepoName         string `json:"repoName,omitempty"`
	RepoHash         string `json:"repoHash,omitempty"`
	OrgHash          string `json:"orgHash,omitempty"`
	Provider         string `json:"provider,omitempty"`
	LastHeadSHA      string `json:"lastHeadSha,omitempty"`
	CurrentHeadSHA   string `json:"currentHeadSha,omitempty"`
	SuggestedCommand string `json:"suggestedCommand,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Suppressed       bool   `json:"suppressed"`
	Linked           bool   `json:"linked"`
	OutOfDate        bool   `json:"outOfDate"`
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

	for _, path := range stateData.SuppressedWorkspaces {
		if samePath(path, absWorkspace) {
			result.Action = ActionSuppressed
			result.Suppressed = true
			result.Reason = "workspace suppressed"
			return result, nil
		}
	}

	repoState, linked := stateData.LinkedRepos[identity.RepoHash]
	result.Linked = linked
	if !linked {
		result.Action = ActionLink
		result.SuggestedCommand = "proofboard link"
		result.Reason = "workspace is not linked"
		return result, nil
	}

	currentHead, err := git.Head(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	result.LastHeadSHA = repoState.LastHeadSHA
	result.CurrentHeadSHA = currentHead

	if repoState.LastHeadSHA == "" || !strings.EqualFold(repoState.LastHeadSHA, currentHead) {
		result.Action = ActionSync
		result.OutOfDate = true
		result.SuggestedCommand = "proofboard sync"
		result.Reason = "workspace has new commits since the last sync"
		return result, nil
	}

	result.Action = ActionNone
	result.Reason = "workspace already linked and synchronized"
	return result, nil
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return aa == bb
}

func (r Result) Marshal() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func (r Result) HumanMessage() string {
	switch r.Action {
	case ActionLink:
		return fmt.Sprintf("Proofboard: New project detected\n%s\nRun `proofboard link` to add it.", r.RepoName)
	case ActionSync:
		return fmt.Sprintf("Proofboard: Project needs sync\n%s\nRun `proofboard sync` to capture the latest work.", r.RepoName)
	case ActionSuppressed:
		return ""
	default:
		return ""
	}
}
