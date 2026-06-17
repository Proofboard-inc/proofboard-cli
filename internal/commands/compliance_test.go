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
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func TestMapTierName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Tier2", "SHA Proof"},
		{"Tier2-skipped", "SHA Proof — handshake skipped"},
		{"Tier1", "Tier1"},
		{"SHA Proof", "SHA Proof"},
	}
	for _, tc := range tests {
		got := mapTierName(tc.input)
		if got != tc.expected {
			t.Errorf("mapTierName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestStatusPendingCheck(t *testing.T) {
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
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test User").Run()
	_ = exec.Command("git", "-C", repoDir, "remote", "add", "origin", "https://github.com/org/repo-status.git").Run()

	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	// Commit a file
	err := os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "add", "hello.txt").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "initial commit").Run()

	cmdHead := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD")
	headBytes, err := cmdHead.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	initialHead := strings.TrimSpace(string(headBytes))

	// Set credentials
	credStore := pbauth.NewCredentialStore(tempHome)
	err = credStore.Save(ctx, model.Credentials{
		Token:     "token-status",
		EmailHash: "email-hash-status",
	})
	if err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	repoHash := crypto.SHA256("github.com:org/repo-status")

	// Set up state
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:          repoHash,
		OrgHash:           crypto.SHA256("github.com:org"),
		PathHash:          "path-hash",
		LastHeadSHA:       initialHead,
		LastSyncAt:        time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		Tier:              "Tier2",
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

	// 1. Check matching HEAD: pending=no, tier mapped to SHA Proof
	{
		var out bytes.Buffer
		cmd := newStatusCommand(ctx, &out)
		err = cmd.ExecuteContext(ctx)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}
		output := out.String()
		expected := fmt.Sprintf("%s tier=SHA Proof lastSync=2026-06-17T12:00:00Z lastHead=%s pending=no\n", repoHash, initialHead)
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got: %q", expected, output)
		}
	}

	// 2. Check differing HEAD: pending=yes
	err = os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello world"), 0o644)
	if err != nil {
		t.Fatalf("failed to edit file: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "add", "hello.txt").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "second commit").Run()

	{
		var out bytes.Buffer
		cmd := newStatusCommand(ctx, &out)
		err = cmd.ExecuteContext(ctx)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}
		output := out.String()
		expectedPrefix := fmt.Sprintf("%s tier=SHA Proof lastSync=2026-06-17T12:00:00Z lastHead=%s pending=yes\n", repoHash, initialHead)
		if !strings.Contains(output, expectedPrefix) {
			t.Errorf("expected output to contain %q, got: %q", expectedPrefix, output)
		}
	}

	// 3. Check not in git repo: pending=unknown
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
		expectedPrefix := fmt.Sprintf("%s tier=SHA Proof lastSync=2026-06-17T12:00:00Z lastHead=%s pending=unknown\n", repoHash, initialHead)
		if !strings.Contains(output, expectedPrefix) {
			t.Errorf("expected output to contain %q, got: %q", expectedPrefix, output)
		}
	}
}

func TestStartupUpdateChecks(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Pre-create ~/.proofboard/dictionary.json
	pbDir := filepath.Join(tempHome, ".proofboard")
	err := os.MkdirAll(pbDir, 0700)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialDict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Docs": {
				Keywords: []string{"readme"},
				Paths:    []string{"*"},
				Impact:   "low",
			},
		},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	err = os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600)
	if err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	// Start a mock HTTP server to return the versions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default handler logic replaced below in actual server instantiation
	}))
	defer srv.Close()

	// Setup handler mapping
	latestCLIVersion := "9.9.9"
	latestDictVersion := "2.0.0"
	dictDownloadURL := srv.URL + "/dictionary/download/dictionary.json"

	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": %q, "url": "https://releases.proofboard.io/9.9.9"}`, latestCLIVersion)
	})
	mux.HandleFunc("/dictionary/latest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": %q, "url": %q}`, latestDictVersion, dictDownloadURL)
	})
	mux.HandleFunc("/dictionary/download/dictionary.json", func(w http.ResponseWriter, r *http.Request) {
		newDict := model.Dictionary{
			Version: latestDictVersion,
			Categories: map[string]model.Signals{
				"Code": {
					Keywords: []string{"func"},
					Paths:    []string{"*.go"},
					Impact:   "high",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newDict)
	})

	srv.Config.Handler = mux

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_DICTIONARY_PATH", "/dictionary/latest.json")

	// Set state with AutoUpdateDictionary=true
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	err = stateStore.Save(ctx, st)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Create a mock command with parent to simulate running a regular command
	parent := &cobra.Command{Use: "proofboard"}
	cmd := &cobra.Command{Use: "status"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	// Execute pre-run checks
	err = runStartupUpdateChecks(ctx, cmd)
	if err != nil {
		t.Fatalf("runStartupUpdateChecks failed: %v", err)
	}

	output := out.String()

	// Verify CLI update message printed
	if !strings.Contains(output, "A new version of the Proofboard CLI is available. Run: proofboard update") {
		t.Errorf("expected output to contain CLI update message, got: %q", output)
	}

	// Verify dictionary updated message printed
	expectedDictMsg := fmt.Sprintf("Dictionary updated successfully to version %s.", latestDictVersion)
	if !strings.Contains(output, expectedDictMsg) {
		t.Errorf("expected output to contain dictionary update message, got: %q", output)
	}

	// Verify state saved with new version
	updatedState, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if updatedState.DictionaryVersion != latestDictVersion {
		t.Errorf("expected state dictionary version to be %q, got %q", latestDictVersion, updatedState.DictionaryVersion)
	}

	// Verify dictionary file on disk updated
	dictData, err := os.ReadFile(filepath.Join(pbDir, "dictionary.json"))
	if err != nil {
		t.Fatalf("read updated dictionary: %v", err)
	}
	var dictObj model.Dictionary
	json.Unmarshal(dictData, &dictObj)
	if dictObj.Version != latestDictVersion {
		t.Errorf("expected dictionary file version to be %q, got %q", latestDictVersion, dictObj.Version)
	}
}

func TestStartupUpdateChecks_SlowNetwork(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Pre-create ~/.proofboard/dictionary.json
	pbDir := filepath.Join(tempHome, ".proofboard")
	err := os.MkdirAll(pbDir, 0700)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialDict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Docs": {
				Keywords: []string{"readme"},
				Paths:    []string{"*"},
				Impact:   "low",
			},
		},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	err = os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600)
	if err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	// Start a mock HTTP server that sleeps for 5 seconds to simulate a slow connection
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_DICTIONARY_PATH", "/dictionary/latest.json")

	// Set state with AutoUpdateDictionary=true
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	err = stateStore.Save(ctx, st)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	start := time.Now()
	// Execute pre-run checks
	err = runStartupUpdateChecks(ctx, cmd)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("runStartupUpdateChecks failed with error: %v, but should have returned nil on timeout", err)
	}

	// The startup checks should time out at 2 seconds and not wait for the 5-second sleep
	if duration >= 4*time.Second {
		t.Errorf("runStartupUpdateChecks took too long: %v, expected less than 4 seconds (timeout is 2s)", duration)
	}

	// Verify no updates happened
	updatedState, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if updatedState.DictionaryVersion != "1.0.0" {
		t.Errorf("expected state dictionary version to remain \"1.0.0\", got %q", updatedState.DictionaryVersion)
	}
}

func TestStartupUpdateChecks_OfflineNetwork(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Use an invalid local address to simulate offline state/network failure
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_DICTIONARY_PATH", "/dictionary/latest.json")

	// Set state with AutoUpdateDictionary=true
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	err := stateStore.Save(ctx, st)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	start := time.Now()
	err = runStartupUpdateChecks(ctx, cmd)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("runStartupUpdateChecks failed with error: %v, but should have returned nil on network failure", err)
	}

	if duration >= 1*time.Second {
		t.Errorf("runStartupUpdateChecks took too long on immediate failure: %v", duration)
	}
}

func TestStartupUpdateChecks_InvalidDictionarySchema(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Pre-create ~/.proofboard/dictionary.json
	pbDir := filepath.Join(tempHome, ".proofboard")
	err := os.MkdirAll(pbDir, 0700)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialDict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Docs": {
				Keywords: []string{"readme"},
				Paths:    []string{"*"},
				Impact:   "low",
			},
		},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	err = os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600)
	if err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replace default handlers below
	}))
	defer srv.Close()

	latestCLIVersion := "1.4.0"
	latestDictVersion := "2.0.0"
	dictDownloadURL := srv.URL + "/dictionary/download/dictionary.json"

	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": %q, "url": "https://releases.proofboard.io/1.4.0"}`, latestCLIVersion)
	})
	mux.HandleFunc("/dictionary/latest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": %q, "url": %q}`, latestDictVersion, dictDownloadURL)
	})
	mux.HandleFunc("/dictionary/download/dictionary.json", func(w http.ResponseWriter, r *http.Request) {
		// Return invalid dictionary (lacking version or categories)
		invalidDict := model.Dictionary{
			Version:    "", // invalid version
			Categories: nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(invalidDict)
	})

	srv.Config.Handler = mux

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_RELEASE_LATEST_DICTIONARY_PATH", "/dictionary/latest.json")

	// Set state with AutoUpdateDictionary=true
	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	st.DictionaryVersion = "1.0.0"
	err = stateStore.Save(ctx, st)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = runStartupUpdateChecks(ctx, cmd)
	if err != nil {
		t.Fatalf("runStartupUpdateChecks failed: %v", err)
	}

	// Verify state version has NOT updated
	updatedState, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if updatedState.DictionaryVersion != "1.0.0" {
		t.Errorf("expected state dictionary version to remain \"1.0.0\", got %q", updatedState.DictionaryVersion)
	}

	// Verify dictionary file on disk version is still 1.0.0
	dictData, err := os.ReadFile(filepath.Join(pbDir, "dictionary.json"))
	if err != nil {
		t.Fatalf("read dictionary: %v", err)
	}
	var dictObj model.Dictionary
	json.Unmarshal(dictData, &dictObj)
	if dictObj.Version != "1.0.0" {
		t.Errorf("expected dictionary file version to remain \"1.0.0\", got %q", dictObj.Version)
	}
}


