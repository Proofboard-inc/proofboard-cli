package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLineInFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")

	updated, err := ensureLineInFile(path, shellDetectionLine)
	if err != nil {
		t.Fatalf("ensureLineInFile first write failed: %v", err)
	}
	if !updated {
		t.Fatalf("expected first write to report updated")
	}

	updated, err = ensureLineInFile(path, shellDetectionLine)
	if err != nil {
		t.Fatalf("ensureLineInFile second write failed: %v", err)
	}
	if updated {
		t.Fatalf("expected second write to be a no-op")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}

	content := string(data)
	if strings.Count(content, shellDetectionLine) != 1 {
		t.Fatalf("expected exactly one detection hook, got content: %q", content)
	}
}

func TestIsInternalCommand(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"notify":          {args: []string{"notify"}, want: true},
		"notify-activate": {args: []string{"notify-activate"}, want: true},
		"hook-maintain":   {args: []string{"hook-maintain"}, want: true},
		"sync":            {args: []string{"sync"}, want: false},
		"empty":           {args: nil, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isInternalCommand(tc.args); got != tc.want {
				t.Fatalf("isInternalCommand(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
