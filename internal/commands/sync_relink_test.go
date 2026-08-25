package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
)

// A session that expires mid-sync reconnects, and the reconnect can land on an
// account that has no project linked for this repository — which is exactly
// what happened to a user whose sync reported:
//
//	Authenticated as <user>. Proofboard Career Agent is connected...
//	transmit sync payload: API returned 400
//
// The backend answers that case with 400 "No linked project found for this
// repository. Run: proofboard link". The CLI knows how to link a repository
// and had just proved it holds working credentials, so failing with a bare
// status code leaves the user to guess at a repair the tool could perform
// itself. Link and retry once instead.
func TestSyncLinksAndRetriesWhenBackendReportsNoLinkedProject(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := createTempGitRepo(t)
	setTestHome(t, homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")

	writeRepoFileAndCommit(t, repoDir)
	ctx := context.Background()
	repo := pbgit.Repo{Path: repoDir}
	head, err := pbgit.Head(ctx, repo)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "update-ref", "refs/remotes/origin/main", head).Run(); err != nil {
		t.Fatalf("update remote ref: %v", err)
	}

	var mu sync.Mutex
	var syncAttempts, linkCalls int
	var linked bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/cli/auth/device-key"):
			_ = json.NewEncoder(w).Encode(map[string]any{"deviceKeyId": "device-key-1"})
		case r.URL.Path == "/api/v1/cli/repos/check":
			// The same backend that rejects the ingest for having no project
			// also reports the repository as unlinked, which is what lets the
			// link flow proceed instead of trusting stale local state.
			mu.Lock()
			isLinked := linked
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"isLinked": isLinked})
		case r.URL.Path == "/api/v1/cli/repos/link":
			mu.Lock()
			linkCalls++
			linked = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isNewProject":      false,
				"projectId":         "project-123",
				"dictionaryVersion": "1.0.0",
				"emailHashKey":      testEmailHashKey,
			})
		case r.URL.Path == "/api/v1/cli/sync":
			mu.Lock()
			syncAttempts++
			isLinked := linked
			mu.Unlock()
			if !isLinked {
				// Exactly what cli-ingest.service.ts returns when no project
				// matches the repository for the authenticated user.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"statusCode": 400,
					"message":    "No linked project found for this repository. Run: proofboard link",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":           "accepted",
				"commitsProcessed": 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)

	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{
		Token: "test-token", EmailHash: "email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	store := state.NewStore(homeDir)
	current := state.Default()
	repoHash := crypto.SHA256("github:org/repo")
	current.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:          repoHash,
		OrgHash:           crypto.SHA256("github:org"),
		ProjectID:         "project-123",
		EmailHashKey:      testEmailHashKey,
		LastSyncAt:        time.Now().Add(-time.Hour),
		DictionaryVersion: version.Version,
	}
	current.AutoUpdateDictionary = false
	if err := store.Save(ctx, current); err != nil {
		t.Fatalf("save state: %v", err)
	}
	restoreWorkingDirectory(t, repoDir)

	var out bytes.Buffer
	cmd := newSyncCommand(ctx, &out)
	cmd.SetArgs(nil)
	syncErr := cmd.ExecuteContext(ctx)

	mu.Lock()
	attempts, links := syncAttempts, linkCalls
	mu.Unlock()

	if links == 0 {
		t.Fatalf("expected the CLI to link the repository after the backend reported no linked project; it never called the link endpoint.\nerror=%v\noutput=%s", syncErr, out.String())
	}
	if syncErr != nil {
		t.Fatalf("expected sync to succeed after linking, got %v\noutput=%s", syncErr, out.String())
	}
	if attempts < 2 {
		t.Fatalf("expected the payload to be retransmitted after linking, saw %d sync attempts", attempts)
	}
	if strings.Contains(out.String(), "API returned 400") {
		t.Fatalf("a repaired sync should not surface a raw status code: %s", out.String())
	}
}
