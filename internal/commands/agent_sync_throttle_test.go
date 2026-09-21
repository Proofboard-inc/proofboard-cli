package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
)

// linkedRepoNeedingSync builds a git repository, links it in state and returns
// the runtime and repository path, so Inspect reports ActionSync for it.
func linkedRepoNeedingSync(t *testing.T, mutate func(*model.LinkedRepoState)) (runtimeContext, string) {
	t.Helper()
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	repoDir := createTempGitRepo(t)
	writeRepoFileAndCommit(t, repoDir)
	ctx := context.Background()

	unlinked, err := detection.Inspect(ctx, homeDir, repoDir, "career-agent")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	store := state.NewStore(homeDir)
	current, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	entry := model.LinkedRepoState{
		RepoHash:          unlinked.RepoHash,
		OrgHash:           unlinked.OrgHash,
		EmailHashKey:      testEmailHashKey,
		DictionaryVersion: version.Version,
	}
	if mutate != nil {
		mutate(&entry)
	}
	current.LinkedRepos[unlinked.RepoHash] = entry
	current.AutoUpdateDictionary = false
	if err := store.Save(ctx, current); err != nil {
		t.Fatalf("save state: %v", err)
	}
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if check, _ := detection.Inspect(ctx, homeDir, repoDir, "career-agent"); check.Action != detection.ActionSync {
		t.Fatalf("fixture: expected ActionSync, got %q", check.Action)
	}
	return runtime, repoDir
}

type launchCounter struct {
	mu    sync.Mutex
	calls []string
}

func (l *launchCounter) launch(_ context.Context, workspace string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, workspace)
	return nil
}

// Editor helper processes each report their own working directory, so one
// repository reaches the agent as several paths — its root and whatever
// subfolders terminals and extensions happen to sit in. The launch throttle was
// keyed by that path, so each got a sync of its own: the log showed the same
// repository transmitted two and three times within one second.
func TestAgentLaunchesOneSyncPerRepositoryNotPerOpenPath(t *testing.T) {
	runtime, repoDir := linkedRepoNeedingSync(t, nil)
	sub := filepath.Join(repoDir, "lib", "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var launches launchCounter
	syncDiscoveredWorkspaces(context.Background(), runtime,
		[]string{repoDir, sub}, map[string]time.Time{}, time.Now(), launches.launch)

	if len(launches.calls) != 1 {
		t.Fatalf("one repository open under %d paths launched %d syncs, want 1: %v",
			2, len(launches.calls), launches.calls)
	}
}

// The agent scans every fifteen seconds and never learned whether a launched
// sync worked, so a payload the server rejects every time was resent about
// once a minute for as long as the editor stayed open — 216 identical 500s for
// one repository in a day and a half.
func TestAgentBacksOffARepositoryWhoseTransmissionsKeepFailing(t *testing.T) {
	now := time.Now()
	runtime, repoDir := linkedRepoNeedingSync(t, func(e *model.LinkedRepoState) {
		e.TransmitFailures = 3
		e.LastTransmitFailureAt = now.Add(-30 * time.Second)
	})

	var launches launchCounter
	syncDiscoveredWorkspaces(context.Background(), runtime,
		[]string{repoDir}, map[string]time.Time{}, now, launches.launch)

	if len(launches.calls) != 0 {
		t.Fatalf("launched %d syncs 30s after the third consecutive failure; want the agent to back off", len(launches.calls))
	}
}

// Guard, not a failing-first test: it passes before and after. Backing off must
// not become a permanent block, or a transient outage would silently stop a
// repository from ever syncing again.
func TestAgentRetriesOnceTheBackoffHasElapsed(t *testing.T) {
	now := time.Now()
	runtime, repoDir := linkedRepoNeedingSync(t, func(e *model.LinkedRepoState) {
		e.TransmitFailures = 3
		e.LastTransmitFailureAt = now.Add(-2 * time.Hour)
	})

	var launches launchCounter
	syncDiscoveredWorkspaces(context.Background(), runtime,
		[]string{repoDir}, map[string]time.Time{}, now, launches.launch)

	if len(launches.calls) != 1 {
		t.Fatalf("launched %d syncs two hours after the last failure, want 1", len(launches.calls))
	}
}

// The agent can only back off if a failed transmission leaves a trace. It used
// to return straight after the error without touching state, so nothing
// distinguished a repository that had just failed from one never tried.
func TestSyncRecordsATransmissionFailureAndClearsItOnSuccess(t *testing.T) {
	var mu sync.Mutex
	failing := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/cli/auth/device-key"):
			_ = json.NewEncoder(w).Encode(map[string]any{"deviceKeyId": "device-key-1"})
		case r.URL.Path == "/api/v1/cli/sync":
			mu.Lock()
			fail := failing
			mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 500, "message": "Internal server error"})
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "commitsProcessed": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)

	runtime, repoDir := linkedRepoNeedingSync(t, nil)
	ctx := context.Background()
	if err := pbauth.NewCredentialStore(runtime.homeDir).Save(ctx, model.Credentials{Token: "test-token", EmailHash: "email-hash"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	restoreWorkingDirectory(t, repoDir)

	repoState := func() model.LinkedRepoState {
		current, err := state.NewStore(runtime.homeDir).Load(ctx)
		if err != nil {
			t.Fatalf("load state: %v", err)
		}
		for _, entry := range current.LinkedRepos {
			return entry
		}
		t.Fatal("no linked repository in state")
		return model.LinkedRepoState{}
	}

	var out bytes.Buffer
	if err := newSyncCommand(ctx, &out).ExecuteContext(ctx); err == nil {
		t.Fatalf("expected the sync to fail against a 500\n%s", out.String())
	}
	after := repoState()
	if after.TransmitFailures != 1 || after.LastTransmitFailureAt.IsZero() {
		t.Fatalf("a failed transmission left no trace: failures=%d lastFailure=%v",
			after.TransmitFailures, after.LastTransmitFailureAt)
	}

	mu.Lock()
	failing = false
	mu.Unlock()
	out.Reset()
	if err := newSyncCommand(ctx, &out).ExecuteContext(ctx); err != nil {
		t.Fatalf("expected the retry to succeed: %v\n%s", err, out.String())
	}
	cleared := repoState()
	if cleared.TransmitFailures != 0 || !cleared.LastTransmitFailureAt.IsZero() {
		t.Fatalf("a successful transmission did not clear the failure record: failures=%d lastFailure=%v",
			cleared.TransmitFailures, cleared.LastTransmitFailureAt)
	}
}
