package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/zalando/go-keyring"
)

const credentialsFileMode os.FileMode = 0o600
const (
	credentialsKeychainService = "Proofboard Career Agent"
	credentialsKeychainAccount = "session-credentials"
)

type credentialSecretStore interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

type systemCredentialSecretStore struct{}

func (systemCredentialSecretStore) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (systemCredentialSecretStore) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

func (systemCredentialSecretStore) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

type CredentialStore struct {
	homeDir     string
	secretStore credentialSecretStore
}

func NewCredentialStore(homeDir string) CredentialStore {
	return CredentialStore{homeDir: homeDir, secretStore: systemCredentialSecretStore{}}
}

// keychainCallTimeout bounds every call into the OS secret store.
//
// These are foreign calls that can block indefinitely, and on macOS they do: a
// session with no unlocked login keychain leaves the Security framework waiting
// inside the syscall with nothing to interrupt it. For the CLI that means a
// freeze after sign-in, credentials in hand and never written, while a working
// file fallback sits a few lines away unreached.
//
// A var so tests can shorten it.
var keychainCallTimeout = 5 * time.Second

// callSecretStore runs one secret-store operation under keychainCallTimeout.
// A call that overruns is abandoned rather than waited on, and reported as a
// failure so the caller takes its fallback path. The goroutine may still be
// parked in the syscall afterwards — there is no way to cancel it — but it no
// longer holds up the command.
func callSecretStore(operation func() error) error {
	result := make(chan error, 1)
	go func() { result <- operation() }()
	select {
	case err := <-result:
		return err
	case <-time.After(keychainCallTimeout):
		return fmt.Errorf("os keychain did not respond within %s", keychainCallTimeout)
	}
}

func (s CredentialStore) Path() string {
	return filepath.Join(s.homeDir, ".proofboard", "credentials.json")
}

// keychainDisabled defaults to false (OS keychain stays the default store).
// The env var remains for test isolation / one-off overrides; the persisted
// state field is the discoverable, documented path via
// `proofboard config set keychain-disabled true`, for environments where OS
// keychain access isn't reachable at all (e.g. some non-standard terminal
// sessions where the OS itself can't locate a keychain for the process).
func (s CredentialStore) keychainDisabled(ctx context.Context) bool {
	if os.Getenv("PROOFBOARD_DISABLE_KEYCHAIN") == "1" {
		return true
	}
	current, err := statestore.NewStore(s.homeDir).Load(ctx)
	if err != nil {
		return false
	}
	return current.KeychainDisabled
}

func (s CredentialStore) Save(ctx context.Context, credentials model.Credentials) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	path := s.Path()
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure credentials directory: %w", err)
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if !s.keychainDisabled(ctx) && s.secretStore != nil {
		setErr := callSecretStore(func() error {
			return s.secretStore.Set(credentialsKeychainService, credentialsKeychainAccount, string(data))
		})
		if setErr == nil {
			// A successful keychain write supersedes the file fallback. Remove
			// any migrated plaintext copy so only the OS-protected secret
			// remains.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove migrated credentials fallback: %w", removeErr)
			}
			return nil
		}
	}
	if err := os.WriteFile(path, data, credentialsFileMode); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Chmod(path, credentialsFileMode); err != nil {
		return fmt.Errorf("secure credentials file: %w", err)
	}
	return nil
}

func (s CredentialStore) Load(ctx context.Context) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("load credentials: %w", err)
	}
	if !s.keychainDisabled(ctx) && s.secretStore != nil {
		var secret string
		getErr := callSecretStore(func() error {
			value, err := s.secretStore.Get(credentialsKeychainService, credentialsKeychainAccount)
			secret = value
			return err
		})
		if getErr == nil {
			var credentials model.Credentials
			if decodeErr := json.Unmarshal([]byte(secret), &credentials); decodeErr != nil {
				return model.Credentials{}, fmt.Errorf("decode credentials from OS keychain: %w", decodeErr)
			}
			return credentials, nil
		}
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

func (s CredentialStore) Delete(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	if !s.keychainDisabled(ctx) && s.secretStore != nil {
		// Best-effort, deliberately not fatal. Save and Load both fall back to
		// the credentials file when no OS keyring is reachable, so making
		// Delete the one operation that hard-fails meant `auth logout` aborted
		// on headless Linux, in containers and over SSH — leaving the
		// credentials it was asked to remove sitting on disk. Removing the
		// file below is what actually logs the machine out.
		_ = callSecretStore(func() error {
			return s.secretStore.Delete(credentialsKeychainService, credentialsKeychainAccount)
		})
	}
	if err := os.Remove(s.Path()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}
