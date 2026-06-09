package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/proofboard/proofboard/internal/model"
)

const credentialsFileMode os.FileMode = 0o600

type CredentialStore struct {
	homeDir string
}

func NewCredentialStore(homeDir string) CredentialStore {
	return CredentialStore{homeDir: homeDir}
}

func (s CredentialStore) Path() string {
	return filepath.Join(s.homeDir, ".proofboard", "credentials.json")
}

func (s CredentialStore) Save(ctx context.Context, credentials model.Credentials) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	path := s.Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.WriteFile(path, data, credentialsFileMode); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

func (s CredentialStore) Load(ctx context.Context) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("load credentials: %w", err)
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return model.Credentials{}, fmt.Errorf("read credentials: %w", err)
	}
	var credentials model.Credentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return model.Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	return credentials, nil
}
