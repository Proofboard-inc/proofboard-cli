package commands

import (
	"bytes"
	"context"
	"encoding/json"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncTransmitsMetadataOnlyUpdate(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	if output, err := exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("initial commit: %v: %s", err, output)
	}
	ctx := context.Background()
	repo := pbgit.Repo{Path: repoDir}
	head, err := pbgit.Head(ctx, repo)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	metadataHash, err := pbgit.MetadataFingerprint(ctx, repo)
	if err != nil {
		t.Fatalf("metadata fingerprint: %v", err)
	}

	var received model.SyncPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/cli/auth/device-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-key-1"})
		case "/api/v1/cli/sync":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatalf("decode sync payload: %v", err)
			}
			_ = json.NewEncoder(w).Encode(model.SyncReceipt{ID: "sync-1", Status: "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)

	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{Token: "token", EmailHash: "email-hash"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	store := state.NewStore(homeDir)
	current := state.Default()
	repoHash := crypto.SHA256("github:org/repo")
	current.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:          repoHash,
		OrgHash:           crypto.SHA256("github:org"),
		LastHeadSHA:       head,
		MetadataHash:      metadataHash,
		ProjectID:         "project-1",
		LastSyncAt:        time.Now().Add(-time.Hour),
		DictionaryVersion: version.Version,
	}
	current.AutoUpdateDictionary = false
	if err := store.Save(ctx, current); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir, "update-ref", "refs/remotes/origin/main", head).Run(); err != nil {
		t.Fatalf("update remote ref: %v", err)
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
	cmd.SetArgs([]string{"--incremental", "--agent"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("metadata sync: %v; output=%s", err, out.String())
	}
	if len(received.SHAs) != 0 {
		t.Fatalf("expected metadata-only payload, got SHAs=%v", received.SHAs)
	}
	if !strings.Contains(out.String(), "Repository metadata synchronized") {
		t.Fatalf("expected metadata sync output, got %q", out.String())
	}
}

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"README.md", true},
		{"docs/API.txt", true},
		{"docs/index.rst", true},
		{"README", true},
		{"CHANGELOG.md", true},
		{"LICENSE", true},
		{"LICENSE-MIT", true},
		{"src/main.go", false},
		{"main.go", false},
		{"README/other.go", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isDocFile(tc.path)
			if got != tc.expected {
				t.Errorf("isDocFile(%q) = %v, want %v", tc.path, got, tc.expected)
			}
		})
	}
}

func TestAbortSync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proofboard-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", tempDir)
	}
	defer os.RemoveAll(tempDir)

	repoHash := "test-repo-hash"
	err = abortSync(tempDir, repoHash)
	if err != nil {
		t.Fatalf("abortSync failed: %v", err)
	}

	logPath := filepath.Join(tempDir, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	logContent := string(data)
	if !strings.Contains(logContent, "trivial merge skipped") {
		t.Errorf("expected log content to contain 'trivial merge skipped', got: %s", logContent)
	}
	if !strings.Contains(logContent, repoHash) {
		t.Errorf("expected log content to contain repo hash %q, got: %s", repoHash, logContent)
	}
}

func TestSyncPipelineOrdering(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/cli/auth/device-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceKeyId": "device-key-pipeline"})
		case "/api/v1/cli/sync":
			_ = json.NewEncoder(w).Encode(model.SyncReceipt{ID: "sync-pipeline", Status: "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(apiServer.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", apiServer.URL)

	// Initialize git repo
	cmdInit := exec.Command("git", "init", "-b", "main")
	cmdInit.Dir = repoDir
	if err := cmdInit.Run(); err != nil {
		cmdInit = exec.Command("git", "init")
		cmdInit.Dir = repoDir
		_ = cmdInit.Run()
	}
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", repoDir, "remote", "add", "origin", "https://github.com/org/repo.git").Run()

	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Setup mock credentials
	credStore := pbauth.NewCredentialStore(tempHome)
	err := credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	})
	if err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Setup mock state with LinkedRepos
	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	repoHash := crypto.SHA256("github:org/repo")
	st.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:  repoHash,
		OrgHash:   crypto.SHA256("github:org"),
		PathHash:  "path-hash",
		ProjectID: "project-pipeline",
	}
	st.AutoUpdateDictionary = false
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Commit at least 2 files in separate commits to avoid single-commit filter
	err = os.WriteFile(filepath.Join(repoDir, "file1.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	exec.Command("git", "-C", repoDir, "add", "file1.go").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "feat: add first file").Run()

	err = os.WriteFile(filepath.Join(repoDir, "file2.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	exec.Command("git", "-C", repoDir, "add", "file2.go").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "feat: add second file").Run()

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
	cmd := newSyncCommand(ctx, &out)

	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("sync pipeline: %v", err)
	}

	// Read sync.log and inspect the steps sequence
	logPath := filepath.Join(tempHome, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read sync.log: %v", err)
	}

	logContent := string(data)
	lines := strings.Split(strings.TrimSpace(logContent), "\n")

	pipelineIndex := -1
	transmitIndex := -1

	for idx, line := range lines {
		if strings.Contains(line, "Phases 2-5: Pipeline") {
			pipelineIndex = idx
		}
		if strings.Contains(line, "Phase 6: transmit") {
			transmitIndex = idx
		}
	}

	if pipelineIndex == -1 {
		t.Errorf("Phases 2-5: Pipeline was not logged in sync.log: %s", logContent)
	}

	if pipelineIndex != -1 && transmitIndex != -1 && pipelineIndex > transmitIndex {
		t.Errorf("Pipeline run happened AFTER Transmit. Pipeline index: %d, Transmit index: %d. Logs: %s", pipelineIndex, transmitIndex, logContent)
	}
}

func TestSyncPrintsProofOfShipEcho(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", tempHome)

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"` + version.Version + `"}`))
		case "/api/v1/cli/auth/device-key":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"deviceKeyId":"device-key-123"}`))
		case "/api/v1/cli/repos/link":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"projectId":"proj-1","dictionaryVersion":"` + version.Version + `","publicKey":"pub-1","isNewProject":false}`))
		case "/api/v1/cli/sync":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST /api/v1/cli/sync, got %s %s", r.Method, r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"sync-1","tier":"sha","status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(apiServer.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", apiServer.URL)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", apiServer.URL)

	ctx := context.Background()

	credStore := pbauth.NewCredentialStore(tempHome)
	if err := credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	}); err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	key, _ := getReadyCareerSummaryMonth(time.Now())
	st.MonthlyCareerSummaryShown[key] = true
	repoPathAbs, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	repoHash := crypto.SHA256("github:org/repo")
	st.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:          repoHash,
		OrgHash:           crypto.SHA256("github:org"),
		PathHash:          crypto.SHA256(repoPathAbs),
		Provider:          "github",
		LastHeadSHA:       "",
		ProjectID:         "proj-1",
		PublicKey:         "pub-1",
		DictionaryVersion: version.Version,
	}
	st.AutoUpdateDictionary = false
	st.FirstRunSetupComplete = true
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	err = os.WriteFile(filepath.Join(repoDir, "file1.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	exec.Command("git", "-C", repoDir, "add", "file1.go").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "feat: add first file").Run()

	err = os.WriteFile(filepath.Join(repoDir, "file2.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	exec.Command("git", "-C", repoDir, "add", "file2.go").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "feat: add second file").Run()

	var out bytes.Buffer
	cmd := newSyncCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("sync execution failed: %v", err)
	}

	if !strings.Contains(out.String(), "Milestone detected") || !strings.Contains(out.String(), "Review | Publish | Ignore") {
		t.Fatalf("expected actionable milestone prompt, got %q", out.String())
	}
}
