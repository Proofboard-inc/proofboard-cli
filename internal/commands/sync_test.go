package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
