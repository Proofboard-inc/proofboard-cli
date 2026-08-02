package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/proofboard/proofboard/internal/api"
	statestore "github.com/proofboard/proofboard/internal/state"
)

func TestDeviceKeyStoreEnsureRegistersReusesAndSigns(t *testing.T) {
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
	tempHome := t.TempDir()
	ctx := context.Background()
	var calls int
	errCh := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cli/auth/device-key" {
			http.NotFound(w, r)
			return
		}
		calls++
		var req struct {
			PublicKey string `json:"publicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errCh <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.PublicKey == "" {
			errCh <- context.Canceled
			http.Error(w, "missing public key", http.StatusBadRequest)
			return
		}
		block, _ := pem.Decode([]byte(req.PublicKey))
		if block == nil || block.Type != "PUBLIC KEY" {
			errCh <- context.Canceled
			http.Error(w, "public key is not PEM", http.StatusBadRequest)
			return
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			errCh <- err
			http.Error(w, "public key is not SPKI", http.StatusBadRequest)
			return
		}
		publicKey, ok := parsed.(*ecdsa.PublicKey)
		if !ok || publicKey.Curve != elliptic.P256() {
			errCh <- context.Canceled
			http.Error(w, "public key is not ECDSA P-256", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-123"})
	}))
	t.Cleanup(server.Close)

	client := api.NewClient(server.URL, "", "", "", "/api/v1/cli/auth/device-key")
	store := NewDeviceKeyStore(tempHome)

	record, err := store.Ensure(ctx, client, "token-123", false)
	if err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}
	if record.DeviceKeyID != "device-123" {
		t.Fatalf("expected registered device key id, got %q", record.DeviceKeyID)
	}
	select {
	case err := <-errCh:
		t.Fatalf("registration handler error: %v", err)
	default:
	}
	if calls != 1 {
		t.Fatalf("expected one registration call, got %d", calls)
	}

	record2, err := store.Ensure(ctx, client, "token-123", false)
	if err != nil {
		t.Fatalf("Ensure reuse failed: %v", err)
	}
	if record2.DeviceKeyID != "device-123" {
		t.Fatalf("expected reused device key id, got %q", record2.DeviceKeyID)
	}
	if calls != 1 {
		t.Fatalf("expected no extra registration calls on reuse, got %d", calls)
	}

	signature, err := store.Sign(ctx, []byte("payload-bytes"))
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	block, _ := pem.Decode([]byte(loaded.PublicKey))
	if block == nil {
		t.Fatal("stored public key is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("stored public key type = %T", parsed)
	}
	digest := sha256.Sum256([]byte("payload-bytes"))
	if !ecdsa.VerifyASN1(publicKey, digest[:], sigBytes) {
		t.Fatalf("signature did not verify with stored public key")
	}

	keyPath := filepath.Join(tempHome, ".proofboard", "device.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("device key file missing: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestDeviceKeyStoreRepairsExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not report POSIX permission bits")
	}
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")
	homeDir := t.TempDir()
	store := NewDeviceKeyStore(homeDir)
	directory := filepath.Dir(store.Path())
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create permissive directory: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("create permissive key file: %v", err)
	}
	record := DeviceKeyRecord{
		DeviceKeyID: "device-1",
		PublicKey:   "public",
		PrivateKey:  "private",
	}
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("Save: %v", err)
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat key directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("key directory mode = %v, want 0700", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

type memoryDeviceKeySecretStore struct {
	value    string
	setCalls int
}

func (s *memoryDeviceKeySecretStore) Get(_, _ string) (string, error) {
	if s.value == "" {
		return "", os.ErrNotExist
	}
	return s.value, nil
}

func (s *memoryDeviceKeySecretStore) Set(_, _, value string) error {
	s.setCalls++
	s.value = value
	return nil
}

// FIX: Ensure() must not re-write the OS keychain on every call once a
// device key is already registered and already loaded from the keychain.
// On macOS, every keyring.Set call can surface an OS access-control prompt,
// so a redundant write on every retryAfterAuth-driven retry (up to 3 per
// sync) manifested as a repeated password/allow dialog per `proofboard sync`.
func TestDeviceKeyStoreEnsureDoesNotRewriteKeychainWhenUnchanged(t *testing.T) {
	tempHome := t.TempDir()
	ctx := context.Background()
	secrets := &memoryDeviceKeySecretStore{}
	store := DeviceKeyStore{homeDir: tempHome, secretStore: secrets}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-123"})
	}))
	t.Cleanup(server.Close)
	client := api.NewClient(server.URL, "", "", "", "/api/v1/cli/auth/device-key")

	if _, err := store.Ensure(ctx, client, "token-123", false); err != nil {
		t.Fatalf("first Ensure failed: %v", err)
	}
	if secrets.setCalls != 1 {
		t.Fatalf("expected exactly one keychain write after registration, got %d", secrets.setCalls)
	}

	for i := 0; i < 3; i++ {
		if _, err := store.Ensure(ctx, client, "token-123", false); err != nil {
			t.Fatalf("repeat Ensure %d failed: %v", i, err)
		}
	}
	if secrets.setCalls != 1 {
		t.Fatalf("expected no additional keychain writes on repeated Ensure calls, got %d total", secrets.setCalls)
	}
}

// FIX: `proofboard config set keychain-disabled true` (persisted in
// state.json) must force device-key storage onto the plaintext file, exactly
// like PROOFBOARD_DISABLE_KEYCHAIN=1, for users whose OS keychain access
// isn't reachable. Default (unset) must keep using the keychain.
func TestDeviceKeyStoreRespectsPersistedKeychainDisabledSetting(t *testing.T) {
	tempHome := t.TempDir()
	ctx := context.Background()
	secrets := &memoryDeviceKeySecretStore{}
	store := DeviceKeyStore{homeDir: tempHome, secretStore: secrets}

	current := statestore.Default()
	current.KeychainDisabled = true
	if err := statestore.NewStore(tempHome).Save(ctx, current); err != nil {
		t.Fatalf("save state: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-456"})
	}))
	t.Cleanup(server.Close)
	client := api.NewClient(server.URL, "", "", "", "/api/v1/cli/auth/device-key")

	if _, err := store.Ensure(ctx, client, "token-123", false); err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}
	if secrets.setCalls != 0 {
		t.Fatalf("expected keychain-disabled setting to skip OS keychain entirely, got %d writes", secrets.setCalls)
	}
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("expected device key file fallback to exist: %v", err)
	}
}

func TestDeviceKeyStorePrefersOSKeychainAndRemovesFallback(t *testing.T) {
	homeDir := t.TempDir()
	secrets := &memoryDeviceKeySecretStore{}
	store := DeviceKeyStore{homeDir: homeDir, secretStore: secrets}
	record := DeviceKeyRecord{
		DeviceKeyID: "device-keychain",
		PublicKey:   "public",
		PrivateKey:  "private",
	}
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if secrets.value == "" {
		t.Fatal("device key was not written to the OS keychain")
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("file fallback exists after keychain save: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != record {
		t.Fatalf("loaded record = %#v, want %#v", loaded, record)
	}
}
