package main

// A stand-in Proofboard backend. It behaves the way the real service is
// documented to behave, and additionally verifies every sync payload signature
// with the public key the Career Agent registered, the same check the real
// backend performs, so a canonicalization mistake fails the test here.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	pbcrypto "github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

type server struct {
	mu                     sync.Mutex
	calls                  []string
	publicKey              *ecdsa.PublicKey
	pollCount              int
	pollsPending           int
	deviceKeyRegistrations int
	deviceKeyID            string
	revokeFirstSync        bool
	firstSyncRejected      bool
	secrets                []string
	email                  string
	syncPayloads           []model.SyncPayload
	failures               []string
	baseURL                string
}

func (s *server) record(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, name)
	fmt.Printf("CALL %s\n", name)
	os.Stdout.Sync()
}

func (s *server) fail(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	s.mu.Lock()
	s.failures = append(s.failures, message)
	s.mu.Unlock()
	fmt.Printf("FAIL %s\n", message)
	os.Stdout.Sync()
}

func (s *server) pass(format string, args ...any) {
	fmt.Printf("PASS %s\n", fmt.Sprintf(format, args...))
	os.Stdout.Sync()
}

// checkNoSecrets enforces the NDA-safe contract: nothing identifying may ever
// cross the network.
func (s *server) checkNoSecrets(path string, body []byte) bool {
	for _, secret := range s.secrets {
		if secret != "" && bytes.Contains(body, []byte(secret)) {
			s.fail("%s leaked %q", path, secret)
			return false
		}
	}
	return true
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if !s.checkNoSecrets(r.URL.Path, body) {
		http.Error(w, "privacy failure", http.StatusInternalServerError)
		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-code":
		s.record("device-code")
		if len(body) > 0 {
			var request map[string]any
			if err := json.Unmarshal(body, &request); err == nil {
				if _, exists := request["deviceCode"]; exists {
					s.fail("device-code request contained a client-generated deviceCode")
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deviceCode":      "secret-polling-token",
			"userCode":        "WXYZ-9876",
			"verificationUrl": s.baseURL + "/cli-auth?code=WXYZ-9876",
			"expiresIn":       600,
		})

	case r.Method == http.MethodGet && r.URL.Path == "/cli-auth":
		// The authorization page. Deliberately carries the framework's
		// not-found text in its payload, the way a real single page
		// application does, to verify the CLI doesn't reject it.
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>` +
			`<title>Proofboard | Verified Career Infrastructure for Engineers</title></head>` +
			`<body><div id="cli-auth">Authorize this device</div>` +
			`<script>self.__next_f.push([1,"This page could not be found"])</script></body></html>`))

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/cli/auth/poll/"):
		s.mu.Lock()
		s.pollCount++
		count := s.pollCount
		pending := s.pollsPending
		s.mu.Unlock()
		if count <= pending {
			// Exercise the polling loop rather than approving instantly.
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
			return
		}
		s.record("poll-approved")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":       "approved",
			"token":        "e2e-access-token",
			"refreshToken": "e2e-refresh-token",
			"username":     "Proofboard Engineer",
		})

	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-key":
		s.record("device-key")
		if r.Header.Get("Authorization") != "Bearer e2e-access-token" {
			s.fail("device-key registration sent %q instead of the access token", r.Header.Get("Authorization"))
		}
		var request struct {
			PublicKey string `json:"publicKey"`
		}
		if err := json.Unmarshal(body, &request); err != nil || request.PublicKey == "" {
			s.fail("device-key registration did not send a public key: %s", body)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		block, _ := pem.Decode([]byte(request.PublicKey))
		if block == nil || block.Type != "PUBLIC KEY" {
			s.fail("device-key public key is not PEM")
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		publicKey, ok := parsed.(*ecdsa.PublicKey)
		if err != nil || !ok || publicKey.Curve != elliptic.P256() {
			s.fail("device-key public key is not ECDSA P-256: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.publicKey = publicKey
		s.deviceKeyRegistrations++
		s.deviceKeyID = fmt.Sprintf("device-key-e2e-%d", s.deviceKeyRegistrations)
		deviceKeyID := s.deviceKeyID
		s.mu.Unlock()
		s.pass("device public key registered (ECDSA P-256 PEM)")
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": deviceKeyID})

	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/repos/link":
		s.record("link")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isNewProject":      true,
			"projectId":         "project-e2e",
			"dictionaryVersion": "1.2.0",
			"publicKey":         "counter-signing-public-key",
			"emailHashKey":      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		})

	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/sync":
		s.record("sync")
		var payload model.SyncPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			s.fail("sync payload did not decode: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		shouldReject := s.revokeFirstSync && !s.firstSyncRejected
		if shouldReject {
			s.firstSyncRejected = true
		}
		s.mu.Unlock()
		if shouldReject {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code":    "DEVICE_KEY_REVOKED",
				"message": "Unknown or revoked device key",
			})
			return
		}
		s.verifySignature(payload)
		s.verifyEmailHash(payload)
		s.mu.Lock()
		s.syncPayloads = append(s.syncPayloads, payload)
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(model.SyncReceipt{ID: "sync-e2e", Status: "ok"})

	case r.URL.Path == "/api/v1/projects/milestone-bundles":
		_ = json.NewEncoder(w).Encode([]any{})

	case r.URL.Path == "/api/v1/cli/dictionary":
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version": "1.2.0",
			"url":     s.baseURL + "/dictionary.json",
		})

	default:
		http.NotFound(w, r)
	}
}

func (s *server) verifyEmailHash(payload model.SyncPayload) {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(s.email))))
	expected := hex.EncodeToString(mac.Sum(nil))
	if payload.EmailHash != expected {
		s.fail("emailHash was not the expected per-project HMAC")
		return
	}
	s.pass("emailHash verified as a per-project HMAC")
}

// verifySignature reproduces the server-side check: rebuild the exact bytes the
// Career Agent signed and verify them against the registered public key.
func (s *server) verifySignature(payload model.SyncPayload) {
	s.mu.Lock()
	publicKey := s.publicKey
	deviceKeyID := s.deviceKeyID
	s.mu.Unlock()

	if payload.DeviceKeyID == "" {
		s.fail("sync payload carried no deviceKeyId")
		return
	}
	if payload.DeviceKeyID != deviceKeyID {
		s.fail("sync payload used stale deviceKeyId")
		return
	}
	if payload.DeviceSignature == "" {
		s.fail("sync payload carried no deviceSignature")
		return
	}
	if publicKey == nil {
		s.fail("sync arrived before any device key was registered")
		return
	}

	signature, err := base64.StdEncoding.DecodeString(payload.DeviceSignature)
	if err != nil {
		s.fail("deviceSignature is not base64: %v", err)
		return
	}

	// The Career Agent signs recursively key-sorted canonical JSON with the
	// signature field omitted.
	signed := payload
	signed.DeviceSignature = ""
	canonical, err := pbcrypto.CanonicalJSON(signed)
	if err != nil {
		s.fail("could not rebuild canonical payload: %v", err)
		return
	}
	digest := sha256.Sum256(canonical)
	if !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		s.fail("SIGNATURE VERIFICATION FAILED — canonical bytes do not match what was signed")
		return
	}
	s.pass("payload signature verified against the registered device key")
}

func main() {
	listenAddress := os.Getenv("MOCK_ADDR")
	if listenAddress == "" {
		listenAddress = "127.0.0.1:0"
	}
	s := &server{
		secrets:         strings.Split(os.Getenv("MOCK_SECRETS"), "|"),
		email:           os.Getenv("MOCK_EMAIL"),
		pollsPending:    2,
		revokeFirstSync: os.Getenv("MOCK_REVOKE_FIRST_SYNC") == "1",
	}

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	s.baseURL = "http://" + listener.Addr().String()
	fmt.Printf("MOCK READY %s\n", s.baseURL)
	os.Stdout.Sync()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	mux.HandleFunc("/__report", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"calls":    s.calls,
			"failures": s.failures,
			"payloads": s.syncPayloads,
		})
	})
	_ = http.Serve(listener, mux)
}
