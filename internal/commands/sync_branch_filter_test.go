package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
)

// Only a git-hook run is limited to production branches. A manual
// `proofboard sync` and the Career Agent send work from any branch, and the
// payload's isDefaultBranch tells the service how to weigh it; filtering
// them as well made a manual sync on a feature branch exit silently having
// sent nothing.
func TestSyncBranchFilterAppliesOnlyToHookRuns(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantTransmit bool
	}{
		{name: "manual", args: []string{"--incremental"}, wantTransmit: true},
		{name: "agent", args: []string{"--incremental", "--agent"}, wantTransmit: true},
		{name: "hook", args: []string{"--incremental", "--from-hook"}, wantTransmit: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			repoDir := createTempGitRepo(t)
			setTestHome(t, homeDir)
			t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
			t.Setenv("PROOFBOARD_DISABLE_STARTUP_CHECKS", "1")

			var syncCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/cli/auth/device-key":
					_, _ = w.Write([]byte(`{"deviceKeyId":"device-key-1"}`))
				case "/api/v1/cli/sync":
					syncCalls.Add(1)
					_, _ = w.Write([]byte(`{"id":"sync-1","status":"ok"}`))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)

			ctx := context.Background()
			if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{Token: "token", EmailHash: "email-hash"}); err != nil {
				t.Fatalf("save credentials: %v", err)
			}
			repoPath, err := filepath.Abs(repoDir)
			if err != nil {
				t.Fatalf("abs: %v", err)
			}
			store := state.NewStore(homeDir)
			current := state.Default()
			repoHash := crypto.SHA256("github:org/repo")
			current.LinkedRepos[repoHash] = model.LinkedRepoState{
				RepoHash:           repoHash,
				OrgHash:            crypto.SHA256("github:org"),
				PathHash:           crypto.SHA256(repoPath),
				Provider:           "github",
				ProjectID:          "project-1",
				EmailHashKey:       testEmailHashKey,
				DictionaryVersion:  version.Version,
				ProductionBranches: []string{"main"},
			}
			current.AutoUpdateDictionary = false
			current.FirstRunSetupComplete = true
			if err := store.Save(ctx, current); err != nil {
				t.Fatalf("save state: %v", err)
			}

			for _, args := range [][]string{
				{"commit", "--allow-empty", "-m", "initial"},
				{"checkout", "-b", "feature/work"},
			} {
				if output, err := exec.Command("git", append([]string{"-C", repoDir}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v: %s", args, err, output)
				}
			}
			if err := os.WriteFile(filepath.Join(repoDir, "feature.go"), []byte("package main\n"), 0o644); err != nil {
				t.Fatalf("write file: %v", err)
			}
			for _, args := range [][]string{
				{"add", "feature.go"},
				{"commit", "-m", "feat: add feature"},
			} {
				if output, err := exec.Command("git", append([]string{"-C", repoDir}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v: %s", args, err, output)
				}
			}

			oldWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("getwd: %v", err)
			}
			if err := os.Chdir(repoDir); err != nil {
				t.Fatalf("chdir: %v", err)
			}
			defer func() { _ = os.Chdir(oldWD) }()

			var out bytes.Buffer
			cmd := newSyncCommand(ctx, &out)
			cmd.SetArgs(tc.args)
			if err := cmd.ExecuteContext(ctx); err != nil {
				t.Fatalf("sync %v: %v; output=%s", tc.args, err, out.String())
			}
			transmitted := syncCalls.Load() > 0
			if transmitted != tc.wantTransmit {
				t.Fatalf("sync %v on a feature branch: transmitted=%v, want %v; output=%s", tc.args, transmitted, tc.wantTransmit, out.String())
			}
		})
	}
}
