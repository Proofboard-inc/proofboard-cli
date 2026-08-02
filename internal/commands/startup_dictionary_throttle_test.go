package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

// RunStartupUpdateChecks (fired via PersistentPreRunE on every
// command — including `sync`, triggered by post-merge/post-pull git hooks)
// must throttle the dictionary version check to at most once per 6h,
// regardless of how many commands run in that window.
func TestStartupUpdateChecksThrottlesDictionaryCheckTo6Hours(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	pbDir := filepath.Join(tempHome, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	initialDict := model.Dictionary{
		Version:    "1.0.0",
		Categories: map[string]model.Signals{"Docs": {Keywords: []string{"readme"}, Impact: "low"}},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	if err := os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600); err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	var dictionaryCheckCount atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version": "1.0.0", "url": ""}`)
	})
	mux.HandleFunc("/dictionary/latest.json", func(w http.ResponseWriter, r *http.Request) {
		dictionaryCheckCount.Add(1)
		fmt.Fprintf(w, `{"version": "1.0.0", "url": ""}`) // no update available
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	newCmd := func() *cobra.Command {
		parent := &cobra.Command{Use: "proofboard"}
		cmd := &cobra.Command{Use: "sync"}
		parent.AddCommand(cmd)
		return cmd
	}

	// First call: LastDictionaryUpdateCheck is zero, so the check fires.
	if err := runStartupUpdateChecks(ctx, newCmd()); err != nil {
		t.Fatalf("first runStartupUpdateChecks: %v", err)
	}
	if got := dictionaryCheckCount.Load(); got != 1 {
		t.Fatalf("expected 1 dictionary check after first call, got %d", got)
	}

	// Second call, immediately after: within the 6h window, must be skipped.
	if err := runStartupUpdateChecks(ctx, newCmd()); err != nil {
		t.Fatalf("second runStartupUpdateChecks: %v", err)
	}
	if got := dictionaryCheckCount.Load(); got != 1 {
		t.Fatalf("expected dictionary check to be throttled (still 1), got %d", got)
	}

	// LastDictionaryUpdateCheck must have been persisted.
	afterFirst, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if afterFirst.LastDictionaryUpdateCheck.IsZero() {
		t.Fatal("expected LastDictionaryUpdateCheck to be persisted after the first check")
	}

	// Simulate 6+ hours passing — a third call must check again.
	afterFirst.LastDictionaryUpdateCheck = time.Now().UTC().Add(-7 * time.Hour)
	if err := stateStore.Save(ctx, afterFirst); err != nil {
		t.Fatalf("save backdated state: %v", err)
	}
	if err := runStartupUpdateChecks(ctx, newCmd()); err != nil {
		t.Fatalf("third runStartupUpdateChecks: %v", err)
	}
	if got := dictionaryCheckCount.Load(); got != 2 {
		t.Fatalf("expected a fresh check after 6h elapsed (count 2), got %d", got)
	}
}

// A failing dictionary check must still be throttled — otherwise a
// flaky/down release server would be hit on every single command with no
// backoff at all.
func TestStartupUpdateChecksThrottlesEvenOnFailure(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	pbDir := filepath.Join(tempHome, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	initialDict := model.Dictionary{
		Version:    "1.0.0",
		Categories: map[string]model.Signals{"Docs": {Keywords: []string{"readme"}, Impact: "low"}},
	}
	initialDictBytes, _ := json.Marshal(initialDict)
	if err := os.WriteFile(filepath.Join(pbDir, "dictionary.json"), initialDictBytes, 0600); err != nil {
		t.Fatalf("write initial dictionary: %v", err)
	}

	var dictionaryCheckCount atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version": "1.0.0", "url": ""}`)
	})
	mux.HandleFunc("/dictionary/latest.json", func(w http.ResponseWriter, r *http.Request) {
		dictionaryCheckCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = true
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	newCmd := func() *cobra.Command {
		parent := &cobra.Command{Use: "proofboard"}
		cmd := &cobra.Command{Use: "sync"}
		parent.AddCommand(cmd)
		return cmd
	}

	if err := runStartupUpdateChecks(ctx, newCmd()); err != nil {
		t.Fatalf("first runStartupUpdateChecks: %v", err)
	}
	if err := runStartupUpdateChecks(ctx, newCmd()); err != nil {
		t.Fatalf("second runStartupUpdateChecks: %v", err)
	}
	if got := dictionaryCheckCount.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt even though it failed (throttle covers attempts, not just successes), got %d", got)
	}
}
