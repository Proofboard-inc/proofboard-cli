package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func localDict(version string) Dictionary {
	return Dictionary{
		Version:    version,
		Categories: map[string]model.Signals{"Docs": {Keywords: []string{"readme"}, Impact: "low"}},
	}
}

func TestUpdateNoOpWhenAlreadyCurrent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cli/dictionary", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version": "1.0.0", "categories": {"Docs": {"keywords": ["readme"], "impact": "low"}}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	homeDir := t.TempDir()
	result, err := Update(context.Background(), homeDir, srv.URL+"/cli/dictionary", localDict("1.0.0"))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.Updated {
		t.Fatal("expected Updated=false when already current")
	}
	if result.Version != "1.0.0" {
		t.Fatalf("expected Version=1.0.0, got %q", result.Version)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".proofboard", "dictionary.json")); err == nil {
		t.Fatal("expected no dictionary.json to be written when no update is needed")
	}
}

func TestUpdateSuccessfullyDownloadsValidatesAndInstalls(t *testing.T) {
	newDict := Dictionary{
		Version:         "2.0.0",
		Categories:      map[string]model.Signals{"Code": {Keywords: []string{"func"}, Impact: "high"}},
		FeatureKeywords: []string{"dashboard", "checkout"},
		StackSignals:    map[string]string{"react": "React"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/cli/dictionary", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(newDict)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	homeDir := t.TempDir()
	result, err := Update(context.Background(), homeDir, srv.URL+"/cli/dictionary", localDict("1.0.0"))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Updated || result.Version != "2.0.0" {
		t.Fatalf("expected an applied update to 2.0.0, got %#v", result)
	}

	targetPath := filepath.Join(homeDir, ".proofboard", "dictionary.json")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read installed dictionary: %v", err)
	}
	var installed Dictionary
	if err := json.Unmarshal(data, &installed); err != nil {
		t.Fatalf("unmarshal installed dictionary: %v", err)
	}
	if installed.Version != "2.0.0" {
		t.Fatalf("installed dictionary version = %q, want 2.0.0", installed.Version)
	}
	if len(installed.FeatureKeywords) != 2 {
		t.Fatalf("expected featureKeywords to round-trip, got %#v", installed.FeatureKeywords)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".proofboard", "dictionary.json.tmp")); err == nil {
		t.Fatal("expected temp file to be renamed away, not left behind")
	}
}

func TestUpdateFetchFailureLeavesExistingDictionaryUntouched(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cli/dictionary", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	homeDir := t.TempDir()
	pbDir := filepath.Join(homeDir, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existingPath := filepath.Join(pbDir, "dictionary.json")
	existingBytes, _ := json.Marshal(localDict("1.0.0"))
	if err := os.WriteFile(existingPath, existingBytes, 0600); err != nil {
		t.Fatalf("write existing dictionary: %v", err)
	}

	_, err := Update(context.Background(), homeDir, srv.URL+"/cli/dictionary", localDict("1.0.0"))
	if err == nil {
		t.Fatal("expected an error when the fetch fails")
	}

	if _, err := os.Stat(filepath.Join(pbDir, "dictionary.json.tmp")); err == nil {
		t.Fatal("expected temp file to be cleaned up after fetch failure")
	}
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing dictionary: %v", err)
	}
	var untouched Dictionary
	if err := json.Unmarshal(data, &untouched); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if untouched.Version != "1.0.0" {
		t.Fatalf("existing dictionary was modified: version = %q", untouched.Version)
	}
}

func TestUpdateValidationFailureLeavesExistingDictionaryUntouched(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cli/dictionary", func(w http.ResponseWriter, r *http.Request) {
		// Newer version but missing required fields (no categories) — fails Validate.
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "2.0.0"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	homeDir := t.TempDir()
	pbDir := filepath.Join(homeDir, ".proofboard")
	if err := os.MkdirAll(pbDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existingPath := filepath.Join(pbDir, "dictionary.json")
	existingBytes, _ := json.Marshal(localDict("1.0.0"))
	if err := os.WriteFile(existingPath, existingBytes, 0600); err != nil {
		t.Fatalf("write existing dictionary: %v", err)
	}

	_, err := Update(context.Background(), homeDir, srv.URL+"/cli/dictionary", localDict("1.0.0"))
	if err == nil {
		t.Fatal("expected an error when the downloaded dictionary fails validation")
	}
	if _, err := os.Stat(filepath.Join(pbDir, "dictionary.json.tmp")); err == nil {
		t.Fatal("expected temp file to be cleaned up after validation failure")
	}
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing dictionary: %v", err)
	}
	var untouched Dictionary
	if err := json.Unmarshal(data, &untouched); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if untouched.Version != "1.0.0" {
		t.Fatalf("existing dictionary was modified: version = %q", untouched.Version)
	}
}

func TestUpdateRejectsNonHTTPSDictionaryURL(t *testing.T) {
	homeDir := t.TempDir()
	_, err := Update(context.Background(), homeDir, "http://example.com/cli/dictionary", localDict("1.0.0"))
	if err == nil {
		t.Fatal("expected an error for a non-HTTPS, non-local dictionary URL")
	}
}
