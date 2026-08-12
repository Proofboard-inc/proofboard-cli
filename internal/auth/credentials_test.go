package auth

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
)

func TestCredentialStoreRepairsExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not report POSIX permission bits")
	}
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
	homeDir := t.TempDir()
	store := NewCredentialStore(homeDir)
	directory := filepath.Dir(store.Path())
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create permissive directory: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{"token":"old"}`), 0o644); err != nil {
		t.Fatalf("create permissive credentials file: %v", err)
	}

	if err := store.Save(context.Background(), model.Credentials{
		Token:        "access-token",
		RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat credentials directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("credentials directory mode = %v, want 0700", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

func TestCredentialStoreDeleteRemovesCredentialsAndIsIdempotent(t *testing.T) {
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")

	ctx := context.Background()
	store := NewCredentialStore(t.TempDir())
	if err := store.Save(ctx, model.Credentials{Token: "access-token"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("credential file still exists after Delete: %v", err)
	}
	if err := store.Delete(ctx); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

type memoryCredentialSecretStore struct {
	value    string
	setCalls int
}

func (s *memoryCredentialSecretStore) Get(_, _ string) (string, error) {
	if s.value == "" {
		return "", os.ErrNotExist
	}
	return s.value, nil
}

func (s *memoryCredentialSecretStore) Set(_, _, value string) error {
	s.setCalls++
	s.value = value
	return nil
}

func (s *memoryCredentialSecretStore) Delete(_, _ string) error {
	// Mirrors go-keyring's own idempotent-delete semantics closely enough for
	// this test double: deleting an already-absent secret is a no-op success,
	// not an error, matching keyring.ErrNotFound handling in CredentialStore.
	s.value = ""
	return nil
}

// FIX: session credentials must prefer the OS keychain over a plaintext file,
// exactly like the device signing key store, and must remove any migrated
// plaintext copy once the keychain write succeeds.
func TestCredentialStorePrefersOSKeychainAndRemovesFallback(t *testing.T) {
	homeDir := t.TempDir()
	secrets := &memoryCredentialSecretStore{}
	store := CredentialStore{homeDir: homeDir, secretStore: secrets}
	credentials := model.Credentials{Token: "access-token", RefreshToken: "refresh-token"}

	if err := store.Save(context.Background(), credentials); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if secrets.value == "" {
		t.Fatal("credentials were not written to the OS keychain")
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("file fallback exists after keychain save: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != credentials {
		t.Fatalf("loaded credentials = %#v, want %#v", loaded, credentials)
	}
}

// FIX: `proofboard config set keychain-disabled true` (persisted in
// state.json) must force credential storage onto the plaintext file, exactly
// like PROOFBOARD_DISABLE_KEYCHAIN=1, for users whose OS keychain access
// isn't reachable. Default (unset) must keep using the keychain.
func TestCredentialStoreRespectsPersistedKeychainDisabledSetting(t *testing.T) {
	homeDir := t.TempDir()
	secrets := &memoryCredentialSecretStore{}
	store := CredentialStore{homeDir: homeDir, secretStore: secrets}

	current := statestore.Default()
	current.KeychainDisabled = true
	if err := statestore.NewStore(homeDir).Save(context.Background(), current); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := store.Save(context.Background(), model.Credentials{Token: "access-token"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if secrets.setCalls != 0 {
		t.Fatalf("expected keychain-disabled setting to skip OS keychain entirely, got %d writes", secrets.setCalls)
	}
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("expected credentials file fallback to exist: %v", err)
	}
}

func TestCredentialStoreDeleteRemovesKeychainEntry(t *testing.T) {
	homeDir := t.TempDir()
	secrets := &memoryCredentialSecretStore{}
	store := CredentialStore{homeDir: homeDir, secretStore: secrets}

	if err := store.Save(context.Background(), model.Credentials{Token: "access-token"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if secrets.value == "" {
		t.Fatal("credentials were not written to the OS keychain")
	}
	if err := store.Delete(context.Background()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if secrets.value != "" {
		t.Fatal("credentials still present in the OS keychain after Delete")
	}
	if err := store.Delete(context.Background()); err != nil {
		t.Fatalf("second Delete should be idempotent: %v", err)
	}
}
