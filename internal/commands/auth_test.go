package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

func TestAuthCommandEndToEnd(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := t.TempDir()

	if err := initGitRepo(repoDir); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	if err := exec.Command("git", "-C", repoDir, "config", "user.email", "Test.User@Example.com").Run(); err != nil {
		t.Fatalf("set git email: %v", err)
	}

	t.Setenv("HOME", tempHome)
	t.Setenv("NO_BROWSER", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pending := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-code":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode device code request: %v", err)
			}
			if _, exists := payload["deviceCode"]; exists {
				t.Fatal("device-code request must not send a client-generated deviceCode")
			}
			pending <- "secret-polling-token"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deviceCode":      "secret-polling-token",
				"userCode":        "ABCD-1234",
				"verificationUrl": "https://proofboard.io/agent/cli-auth?code=ABCD-1234",
				"expiresIn":       600,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-key":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"deviceKeyId": "device-key-123",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/cli/auth/poll/"):
			deviceCode := strings.TrimPrefix(r.URL.Path, "/api/v1/cli/auth/poll/")
			select {
			case expected := <-pending:
				if deviceCode != expected {
					t.Fatalf("poll device code mismatch: got %s want %s", deviceCode, expected)
				}
			default:
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":       "approved",
				"token":        "jwt-token-123",
				"refreshToken": "refresh-token-456",
				"username":     "Ada Lovelace",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", server.URL+"/agent/cli-auth")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	var out bytes.Buffer
	cmd := newAuthCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("auth command failed: %v\noutput: %s", err, out.String())
	}

	output := out.String()
	if !strings.Contains(output, "Authenticated as Ada Lovelace") {
		t.Fatalf("expected success output, got: %q", output)
	}

	credPath := filepath.Join(tempHome, ".proofboard", "credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}

	var creds model.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		t.Fatalf("decode credentials: %v", err)
	}
	if creds.Token != "jwt-token-123" || creds.RefreshToken != "refresh-token-456" || creds.Username != "Ada Lovelace" {
		t.Fatalf("unexpected stored credentials: %+v", creds)
	}
	expectedEmailHash := crypto.NormalizedSHA256("Test.User@Example.com")
	if creds.EmailHash != expectedEmailHash {
		t.Fatalf("unexpected stored email hash: got %s want %s", creds.EmailHash, expectedEmailHash)
	}

	if info, err := os.Stat(credPath); err != nil {
		t.Fatalf("stat credentials: %v", err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected credentials file mode: %v", info.Mode().Perm())
	}
}

func initGitRepo(dir string) error {
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("git", "init")
	cmd.Dir = dir
	return cmd.Run()
}
