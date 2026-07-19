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

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
)

func TestDetectCommandAutolinksSilently(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", tempHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cli/repos/link":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode link request: %v", err)
			}
			if req["createNew"] != true {
				t.Fatalf("expected createNew true, got %v", req["createNew"])
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isNewProject":      true,
				"projectId":         "project-1",
				"publicKey":         "public-key",
				"dictionaryVersion": "v1",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", server.URL)

	ctx := context.Background()
	credStore := pbauth.NewCredentialStore(tempHome)
	if err := credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# proofboard\n"), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "README.md").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "feat: initial commit").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	var out bytes.Buffer
	cmd := newDetectCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("detect command failed: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected silent detect command, got output: %q", out.String())
	}

	st, err := state.NewStore(tempHome).Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repoHash := crypto.SHA256("github:org/repo")
	if _, ok := st.LinkedRepos[repoHash]; !ok {
		logPath := filepath.Join(tempHome, ".proofboard", "sync.log")
		if data, readErr := os.ReadFile(logPath); readErr == nil {
			t.Fatalf("expected repo %s to be linked after detect; sync.log:\n%s", repoHash, string(data))
		}
		t.Fatalf("expected repo %s to be linked after detect", repoHash)
	}

	logPath := filepath.Join(tempHome, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read sync log: %v", err)
	}
	if !strings.Contains(string(data), "detect") {
		t.Fatalf("expected detect entry in sync.log, got: %s", string(data))
	}
}
