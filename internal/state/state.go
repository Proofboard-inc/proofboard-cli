package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

type Store struct {
	homeDir string
}

func NewStore(homeDir string) Store {
	return Store{homeDir: homeDir}
}

func (s Store) Path() string {
	return filepath.Join(s.homeDir, ".proofboard", "state.json")
}

func (s Store) Save(ctx context.Context, state model.State) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	path := s.Path()
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure state file: %w", err)
	}
	return nil
}

func (s Store) Load(ctx context.Context) (model.State, error) {
	if err := ctx.Err(); err != nil {
		return model.State{}, fmt.Errorf("load state: %w", err)
	}
	data, err := os.ReadFile(s.Path())
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return model.State{}, fmt.Errorf("read state: %w", err)
	}
	var state model.State
	if err := json.Unmarshal(data, &state); err != nil {
		return model.State{}, fmt.Errorf("decode state: %w", err)
	}
	if state.LinkedRepos == nil {
		state.LinkedRepos = make(map[string]model.LinkedRepoState)
	}
	if state.SuppressedWorkspaces == nil {
		state.SuppressedWorkspaces = make([]string, 0)
	}
	if state.PromptedWorkspaces == nil {
		state.PromptedWorkspaces = make(map[string]time.Time)
	}
	migratedSuppressions := make([]string, 0, len(state.SuppressedWorkspaces))
	suppressionSeen := make(map[string]bool)
	suppressionStateChanged := false
	for _, value := range state.SuppressedWorkspaces {
		key := value
		if !isSHA256(value) {
			var migrationErr error
			key, migrationErr = WorkspaceSuppressionKey(value)
			if migrationErr != nil {
				suppressionStateChanged = true
				continue
			}
			suppressionStateChanged = true
		}
		if suppressionSeen[key] {
			suppressionStateChanged = true
			continue
		}
		suppressionSeen[key] = true
		migratedSuppressions = append(migratedSuppressions, key)
	}
	state.SuppressedWorkspaces = migratedSuppressions
	if state.IDEProcesses == nil {
		state.IDEProcesses = Default().IDEProcesses
	}
	if suppressionStateChanged {
		if err := s.Save(ctx, state); err != nil {
			return model.State{}, fmt.Errorf("migrate workspace suppression state: %w", err)
		}
	}
	return state, nil
}

func Default() model.State {
	return model.State{
		LinkedRepos:          make(map[string]model.LinkedRepoState),
		WatchedBranches:      []string{"main", "master", "develop"},
		AutoUpdateDictionary: true,
		SuppressedWorkspaces: make([]string, 0),
		IDEProcesses:         []string{"code", "code-insiders", "cursor", "webstorm", "idea", "zed", "sublime_text", "vim", "nvim"},
	}
}

func WorkspaceSuppressionKey(workspace string) (string, error) {
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace suppression path: %w", err)
	}
	// Resolve symlinks so this always hashes the same real path git itself
	// would report as the repo root (`git rev-parse --show-toplevel`
	// resolves symlinks). On macOS in particular, a path can round-trip
	// through a symlinked temp/mount point (e.g. /var/folders/... is a
	// symlink to /private/var/folders/...) — without this, a suppression
	// recorded via one form of the path silently never matches a lookup via
	// the other, and workspace suppression stops working. Falls back to the
	// unresolved absolute path if the workspace doesn't exist yet (e.g. it
	// was just deleted) rather than failing outright.
	resolved, evalErr := filepath.EvalSymlinks(absolute)
	if evalErr == nil {
		absolute = resolved
	}
	normalized := filepath.Clean(absolute)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return crypto.SHA256(normalized), nil
}

func IsWorkspaceSuppressed(state model.State, workspace string) bool {
	key, err := WorkspaceSuppressionKey(workspace)
	if err != nil {
		return false
	}
	for _, existing := range state.SuppressedWorkspaces {
		if existing == key {
			return true
		}
	}
	return false
}

func AddWorkspaceSuppression(state model.State, workspace string) (model.State, error) {
	key, err := WorkspaceSuppressionKey(workspace)
	if err != nil {
		return state, err
	}
	for _, existing := range state.SuppressedWorkspaces {
		if existing == key {
			return state, nil
		}
	}
	state.SuppressedWorkspaces = append(state.SuppressedWorkspaces, key)
	return state, nil
}

// WasWorkspacePrompted reports whether the workspace connection prompt has
// already been shown for a workspace. The prompt is shown at most once per
// workspace: opening an editor or a terminal must not re-ask on every session.
func WasWorkspacePrompted(state model.State, workspace string) bool {
	key, err := WorkspaceSuppressionKey(workspace)
	if err != nil {
		return false
	}
	_, prompted := state.PromptedWorkspaces[key]
	return prompted
}

// RecordWorkspacePrompt remembers that the prompt was shown for a workspace.
func RecordWorkspacePrompt(state model.State, workspace string, at time.Time) (model.State, error) {
	key, err := WorkspaceSuppressionKey(workspace)
	if err != nil {
		return state, err
	}
	if state.PromptedWorkspaces == nil {
		state.PromptedWorkspaces = make(map[string]time.Time)
	}
	if _, exists := state.PromptedWorkspaces[key]; exists {
		return state, nil
	}
	state.PromptedWorkspaces[key] = at.UTC()
	return state, nil
}

// ClearWorkspacePrompt forgets a recorded prompt so the workspace can be
// offered again, which is what disconnecting a project should do.
func ClearWorkspacePrompt(state model.State, workspace string) (model.State, error) {
	key, err := WorkspaceSuppressionKey(workspace)
	if err != nil {
		return state, err
	}
	delete(state.PromptedWorkspaces, key)
	return state, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
