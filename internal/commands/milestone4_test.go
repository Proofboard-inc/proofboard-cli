package commands

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
)

func TestUpdateDictionaryCommand_Success(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Spin up mock release server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dictionary/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version": "1.3.0", "url": "/dictionary/1.3.0/dictionary.json"}`))
		case "/dictionary/1.3.0/dictionary.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"version": "1.3.0",
				"categories": {
					"Authentication & security": {
						"impact": "feature",
						"keyword_signals": ["auth"],
						"path_signals": ["auth/"]
					}
				}
			}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", ts.URL)
	t.Setenv("PROOFBOARD_API_BASE_URL", ts.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	var out bytes.Buffer
	cmd := newUpdateDictionaryCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("update-dictionary command failed: %v", err)
	}

	expectedMsg := "Dictionary updated successfully to version 1.3.0.\n"
	if out.String() != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, out.String())
	}

	// Verify target dictionary file is written and contains correct content
	targetPath := filepath.Join(tempHome, ".proofboard", "dictionary.json")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target dictionary file: %v", err)
	}

	var loaded dictData
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to decode target dictionary: %v", err)
	}
	if loaded.Version != "1.3.0" {
		t.Errorf("expected dictionary version 1.3.0, got %q", loaded.Version)
	}

	// Verify state.json was updated with correct dictionary version
	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if st.DictionaryVersion != "1.3.0" {
		t.Errorf("expected state dictionaryVersion to be 1.3.0, got %q", st.DictionaryVersion)
	}
}

type dictData struct {
	Version    string                 `json:"version"`
	Categories map[string]interface{} `json:"categories"`
}

func TestUpdateDictionaryCommand_SchemaCheckFailure(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	// Spin up mock release server returning invalid schema (empty version)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dictionary/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version": "1.4.0", "url": "/dictionary/1.4.0/dictionary.json"}`))
		case "/dictionary/1.4.0/dictionary.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"version": "",
				"categories": {}
			}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", ts.URL)
	t.Setenv("PROOFBOARD_API_BASE_URL", ts.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	var out bytes.Buffer
	cmd := newUpdateDictionaryCommand(ctx, &out)
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected error due to schema validation failure, but got nil")
	}

	if !strings.Contains(err.Error(), "validate downloaded dictionary") {
		t.Errorf("expected validation error message, got: %v", err)
	}

	// Verify target dictionary file was not created or modified to invalid one
	targetPath := filepath.Join(tempHome, ".proofboard", "dictionary.json")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("expected dictionary.json file to not exist due to failed update")
	}

	// Verify state.json dictionary version was not updated
	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if st.DictionaryVersion == "1.4.0" {
		t.Error("state.json dictionaryVersion should not be 1.4.0 after failure")
	}
}

func TestUpdateCommand_BinaryReplacement(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	binaryName := fmt.Sprintf("proofboard-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix)

	// Spin up mock release server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version": "1.3.0", "url": "/1.3.0/` + binaryName + `"}`))
		case "/1.3.0/" + binaryName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("mock binary content payload"))
		case "/1.3.0/" + binaryName + ".sig":
			w.Header().Set("Content-Type", "application/octet-stream")
			// Signature for "mock binary content payload" using proofboard_private.pem
			sigBytes, _ := hex.DecodeString("3045022100cb978a826f9edb1f110438413354edcfa054fdba1fee628b1861442c31d86d1c0220009e5eebf0397421c09999e59d632826cc3234168154f9f8cd2b35f7765a2cd2")
			_, _ = w.Write(sigBytes)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", ts.URL)

	execPath := filepath.Join(t.TempDir(), "proofboard")
	if err := os.WriteFile(execPath, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("write temporary executable: %v", err)
	}
	installCalled := false

	var out bytes.Buffer
	cmd := newUpdateCommandWithOptions(ctx, &out, updateCommandOptions{
		executablePath: func() (string, error) { return execPath, nil },
		install: func(io.Writer) error {
			installCalled = true
			return nil
		},
	})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("update command failed: %v", err)
	}
	if !installCalled {
		t.Fatal("expected updater to invoke installation after replacement")
	}

	expectedMsg := "Proofboard Career Agent updated successfully to version 1.3.0.\n"
	if !strings.Contains(out.String(), expectedMsg) {
		t.Errorf("expected message to contain %q, got %q", expectedMsg, out.String())
	}

	// Verify that the executable file content was replaced with our mock binary content
	newBytes, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("failed to read modified executable: %v", err)
	}
	if string(newBytes) != "mock binary content payload" {
		t.Errorf("expected replaced executable content to be 'mock binary content payload', got %q", string(newBytes))
	}
}

func TestSyncCommand_LogsActivity(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)

	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	credStore := pbauth.NewCredentialStore(tempHome)
	err := credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	})
	if err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Make sure we have a config set up
	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Pre-link repo in state
	_, _ = filepath.Abs(repoDir)
	repoHash := "test-repo-hash"
	st.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:    repoHash,
		PathHash:    "path-hash",
		LastHeadSHA: "some-sha",
	}
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Write a file and commit it to git
	err = os.WriteFile(filepath.Join(repoDir, "file.go"), []byte("package main"), 0o644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	exec.Command("git", "-C", repoDir, "add", "file.go").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "initial commit").Run()

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

	// Execute command (will fail on remote handshake or other parts since no mock API,
	// but should still log activity!)
	_ = cmd.ExecuteContext(ctx)

	// Verify that sync.log is created in tempHome
	logPath := filepath.Join(tempHome, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read sync.log: %v", err)
	}

	logContent := string(data)
	if !strings.Contains(logContent, "start") {
		t.Errorf("expected log content to record sync start, got: %s", logContent)
	}

	// Verify that log lines contain only NDA-compliant fields (i.e. no author, no message, no path)
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	for _, line := range lines {
		parts := strings.Split(line, " — ")
		if len(parts) < 5 {
			t.Errorf("log line should have at least 5 parts, got: %q", line)
		}
		// Confirm no sensitive strings
		sensitive := []string{"test@example.com", "initial commit", "file.go"}
		for _, s := range sensitive {
			if strings.Contains(line, s) {
				t.Errorf("sensitive leak detected: log line contains %q", s)
			}
		}
	}
}
