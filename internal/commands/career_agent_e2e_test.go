package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
)

func TestCareerAgentEndToEndAuthorizesConnectsAndSyncs(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("NO_BROWSER", "1")
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	secretSubject := "feat: Confidential Ledger Launch"
	secretPath := "clients/confidential-ledger/launch.go"
	if err := os.MkdirAll(filepath.Join(repoDir, filepath.Dir(secretPath)), 0o700); err != nil {
		t.Fatalf("create repository directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, secretPath), []byte("package confidentialledger\n"), 0o600); err != nil {
		t.Fatalf("write repository file: %v", err)
	}
	runGit(t, repoDir, "add", secretPath)
	runGit(t, repoDir, "commit", "-m", secretSubject)
	head := gitOutput(t, repoDir, "rev-parse", "HEAD")
	runGit(t, repoDir, "update-ref", "refs/remotes/origin/main", head)
	runGit(t, repoDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	var (
		server *httptest.Server
		mu     sync.Mutex
		calls  []string
	)
	recordCall := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, value)
	}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read %s request: %v", r.URL.Path, err)
			http.Error(w, "request read failed", http.StatusInternalServerError)
			return
		}
		for _, secret := range []string{secretSubject, secretPath, "org/repo", "test@example.com"} {
			if bytes.Contains(body, []byte(secret)) {
				t.Errorf("%s request exposed %q: %s", r.URL.Path, secret, body)
				http.Error(w, "privacy failure", http.StatusInternalServerError)
				return
			}
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-code":
			recordCall("authorize")
			if len(body) > 0 {
				var request map[string]any
				if err := json.Unmarshal(body, &request); err != nil {
					t.Errorf("decode device-code request: %v", err)
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if _, exists := request["deviceCode"]; exists {
					t.Error("device-code request sent a client-generated deviceCode")
					http.Error(w, "forbidden field", http.StatusBadRequest)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deviceCode":      "polling-token",
				"userCode":        "ABCD-1234",
				"verificationUrl": server.URL + "/agent/cli-auth?code=ABCD-1234",
				"expiresIn":       60,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/agent/cli-auth":
			if r.URL.Query().Get("code") != "ABCD-1234" {
				t.Errorf("authorization page code = %q", r.URL.Query().Get("code"))
			}
			_, _ = w.Write([]byte("Authorize Proofboard Career Agent"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cli/auth/poll/polling-token":
			recordCall("exchange")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":       "approved",
				"token":        "access-token",
				"refreshToken": "months-long-refresh-token",
				"username":     "Proofboard Engineer",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/auth/device-key":
			recordCall("device-key")
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-e2e"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/repos/link":
			recordCall("connect")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isNewProject":      false,
				"projectId":         "project-e2e",
				"dictionaryVersion": version.Version,
				"emailHashKey":      testEmailHashKey,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/sync":
			recordCall("sync")
			var payload model.SyncPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("decode sync payload: %v", err)
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			if len(payload.SHAs) != 1 || payload.SHAs[0] != head {
				t.Errorf("sync SHAs = %#v, want %s", payload.SHAs, head)
			}
			_ = json.NewEncoder(w).Encode(model.SyncReceipt{ID: "sync-e2e", Status: "ok"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/milestone-bundles":
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", server.URL+"/agent/cli-auth")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	current := state.Default()
	current.AutoUpdateDictionary = false
	if err := state.NewStore(homeDir).Save(ctx, current); err != nil {
		t.Fatalf("save initial state: %v", err)
	}
	restoreWorkingDirectory(t, repoDir)

	var out bytes.Buffer
	command := newSyncCommand(ctx, &out)
	command.SetArgs([]string{"--agent"})
	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatalf("Career Agent sync: %v\n%s", err, out.String())
	}

	mu.Lock()
	gotCalls := append([]string(nil), calls...)
	mu.Unlock()
	wantCalls := []string{"authorize", "exchange", "device-key", "connect", "sync"}
	if strings.Join(gotCalls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("Career Agent sequence = %#v, want %#v", gotCalls, wantCalls)
	}

	credentials, err := pbauth.NewCredentialStore(homeDir).Load(ctx)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if credentials.Token != "access-token" ||
		credentials.RefreshToken != "months-long-refresh-token" ||
		credentials.EmailHash != crypto.NormalizedSHA256("test@example.com") {
		t.Fatalf("stored credentials = %+v", credentials)
	}
	persisted, err := state.NewStore(homeDir).Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repoHash := crypto.SHA256("github:org/repo")
	repoState, linked := persisted.LinkedRepos[repoHash]
	if !linked || repoState.ProjectID != "project-e2e" || repoState.EmailHashKey != testEmailHashKey ||
		repoState.LastHeadSHA != head || repoState.LastSyncAt.IsZero() {
		t.Fatalf("tracked project state = %+v, linked=%v", repoState, linked)
	}
}
