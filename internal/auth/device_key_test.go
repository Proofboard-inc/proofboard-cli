package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/proofboard/proofboard/internal/api"
)

func TestDeviceKeyStoreEnsureRegistersReusesAndSigns(t *testing.T) {
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
	publicBytes, err := base64.StdEncoding.DecodeString(loaded.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicBytes), []byte("payload-bytes"), sigBytes) {
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
