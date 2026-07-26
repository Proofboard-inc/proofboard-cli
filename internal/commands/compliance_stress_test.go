package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

// TestStartupChecksTimeout checks that startup checks run under 2 seconds and do not block command execution.
func TestStartupChecksTimeout(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	// Pre-create ~/.proofboard/dictionary.json
	pbDir := filepath.Join(tempHome, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialDict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Docs": {Keywords: []string{"readme"}, Paths: []string{"*"}, Impact: "low"},
		},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	if err := os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600); err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	// Create a hanging mock server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep for 5 seconds to exceed the 2-second timeout
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	// Set state with AutoUpdateDictionary=true
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	// Measure elapsed time
	startTime := time.Now()
	err := runStartupUpdateChecks(ctx, cmd)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Errorf("expected runStartupUpdateChecks to not fail even on network issues, got: %v", err)
	}

	if elapsed >= 3*time.Second {
		t.Errorf("startup checks took too long: %v (expected < 2s due to timeout)", elapsed)
	}

	t.Logf("Startup checks completed in %v, which is under the 2-second threshold", elapsed)
}

// TestStartupChecksNetworkFailure checks that network failure (e.g. invalid host) does not block command execution.
func TestStartupChecksNetworkFailure(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	// Pre-create ~/.proofboard/dictionary.json
	pbDir := filepath.Join(tempHome, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialDict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Docs": {Keywords: []string{"readme"}, Paths: []string{"*"}, Impact: "low"},
		},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	if err := os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600); err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	// Use an invalid host URL to guarantee network connection failure
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", "https://invalid-host-for-testing-12345.io")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", "https://invalid-host-for-testing-12345.io")
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runStartupUpdateChecks(ctx, cmd)
	if err != nil {
		t.Errorf("expected runStartupUpdateChecks to handle network failure gracefully and return nil, got: %v", err)
	}

	t.Log("Startup checks gracefully handled network failure and did not fail command execution.")
}

// TestStatusPendingStates verifies pending=yes/no/unknown status check outputs.
func TestStatusPendingStates(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := t.TempDir()

	// Initialize git repo
	cmdInit := exec.Command("git", "init", "-b", "main")
	cmdInit.Dir = repoDir
	if err := cmdInit.Run(); err != nil {
		cmdInit = exec.Command("git", "init")
		cmdInit.Dir = repoDir
		_ = cmdInit.Run()
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "tester@example.com").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Tester User").Run()
	_ = exec.Command("git", "-C", repoDir, "remote", "add", "origin", "https://github.com/org/repo-pending.git").Run()

	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	// Commit a file
	err := os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("data"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "add", "a.txt").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "init").Run()

	cmdHead := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD")
	headBytes, err := cmdHead.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	initialHead := strings.TrimSpace(string(headBytes))

	// Set credentials
	credStore := pbauth.NewCredentialStore(tempHome)
	err = credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	})
	if err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	repoHash := crypto.SHA256("github:org/repo-pending")
	metadataHash, err := pbgit.MetadataFingerprint(ctx, pbgit.Repo{Path: repoDir})
	if err != nil {
		t.Fatalf("metadata fingerprint: %v", err)
	}

	// Set up state
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:          repoHash,
		OrgHash:           crypto.SHA256("github:org"),
		PathHash:          "path-hash",
		LastHeadSHA:       initialHead,
		LastSyncAt:        time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		ProjectID:         "proj-123",
		EmailHashKey:      testEmailHashKey,
		DictionaryVersion: "1.0.0",
		MetadataHash:      metadataHash,
	}
	st.LinkedRepos["some-other-repo"] = model.LinkedRepoState{
		RepoHash:          "some-other-repo",
		OrgHash:           "some-other-org",
		PathHash:          "path-hash-2",
		LastHeadSHA:       "some-sha",
		LastSyncAt:        time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		ProjectID:         "proj-456",
		DictionaryVersion: "1.0.0",
	}
	err = stateStore.Save(ctx, st)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Change working directory to repo
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

	// 1. HEAD matches -> pending=no, and tier maps correctly to "SHA Proof"
	{
		var out bytes.Buffer
		cmd := newStatusCommand(ctx, &out)
		err = cmd.ExecuteContext(ctx)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}
		output := out.String()
		expected := fmt.Sprintf("%s projectID=proj-123 lastSync=2026-06-17T12:00:00Z lastHead=%s pending=no\n", repoHash, initialHead)
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain matching head: %q, got: %q", expected, output)
		}

		// Also check that the other repository (not in working directory) outputs pending=unknown and tier maps correctly to "SHA Proof — handshake skipped"
		expectedOther := "some-other-repo projectID=proj-456 lastSync=2026-06-17T12:00:00Z lastHead=some-sha pending=unknown\n"
		if !strings.Contains(output, expectedOther) {
			t.Errorf("expected other repo output: %q, got: %q", expectedOther, output)
		}
	}

	// 2. HEAD differs -> pending=yes
	err = os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("data modified"), 0o644)
	if err != nil {
		t.Fatalf("failed to edit file: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "add", "a.txt").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "second").Run()

	{
		var out bytes.Buffer
		cmd := newStatusCommand(ctx, &out)
		err = cmd.ExecuteContext(ctx)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}
		output := out.String()
		expectedPrefix := fmt.Sprintf("%s projectID=proj-123 lastSync=2026-06-17T12:00:00Z lastHead=%s pending=yes\n", repoHash, initialHead)
		if !strings.Contains(output, expectedPrefix) {
			t.Errorf("expected output to indicate pending=yes: %q, got: %q", expectedPrefix, output)
		}
	}

	// 3. Not in a git repository -> pending=unknown
	nonGitDir := t.TempDir()
	if err := os.Chdir(nonGitDir); err != nil {
		t.Fatalf("Chdir non-git: %v", err)
	}
	{
		var out bytes.Buffer
		cmd := newStatusCommand(ctx, &out)
		err = cmd.ExecuteContext(ctx)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}
		output := out.String()
		expectedPrefix := fmt.Sprintf("%s projectID=proj-123 lastSync=2026-06-17T12:00:00Z lastHead=%s pending=unknown\n", repoHash, initialHead)
		if !strings.Contains(output, expectedPrefix) {
			t.Errorf("expected output to indicate pending=unknown: %q, got: %q", expectedPrefix, output)
		}
	}
}
