package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
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
	return state, nil
}

func Default() model.State {
	return model.State{
		LinkedRepos:          make(map[string]model.LinkedRepoState),
		WatchedBranches:      []string{"main", "master", "production"},
		AutoUpdateDictionary: true,
	}
}
