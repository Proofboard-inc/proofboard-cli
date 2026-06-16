package phase2

import (
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestClassifyAndNoiseScore(t *testing.T) {
	dict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Feat": {
				Keywords: []string{"Feature", "feat"},
				Paths:    []string{"cmd/", "internal/"},
				Impact:   "feature",
			},
			"Fix": {
				Keywords: []string{"bug", "fix"},
				Paths:    []string{"pkg/"},
				Impact:   "bugfix",
			},
		},
	}

	raw := []model.RawCommit{
		{
			SHA:          "sha1",
			Timestamp:    time.Now(),
			Additions:    10,
			Deletions:    5,
			FilesChanged: 2,
			Subject:      []byte("Implement new feature for auth"),
			FilePaths:    []string{"cmd/auth.go", "internal/auth.go"},
			AuthorEmail:  "user@example.com",
			Repository:   "repo",
			Organization: "org",
		},
		{
			SHA:          "sha2",
			Timestamp:    time.Now(),
			Additions:    1,
			Deletions:    1,
			FilesChanged: 1,
			Subject:      []byte("wip"),
			FilePaths:    []string{"pkg/other.go"},
			AuthorEmail:  "user@example.com",
			Repository:   "repo",
			Organization: "org",
		},
	}

	origSubj1 := raw[0].Subject
	origSubj2 := raw[1].Subject

	signals := Classify(raw, dict)

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	// First commit: "Implement new feature for auth" -> Category Feat (keyword "feature", paths cmd/, internal/)
	if signals[0].PrimaryCategory != "Feat" {
		t.Errorf("expected Feat category, got %s", signals[0].PrimaryCategory)
	}
	if signals[0].NoiseScore >= 0.5 {
		t.Errorf("expected low noise score for feature commit, got %f", signals[0].NoiseScore)
	}

	// Second commit: "wip" -> NoiseScore should be 0.95 (trivial keyword)
	if signals[1].NoiseScore != 0.95 {
		t.Errorf("expected noise score 0.95 for wip, got %f", signals[1].NoiseScore)
	}

	// Ensure inputs were zeroed and nil'd
	if raw[0].Subject != nil {
		t.Errorf("expected subject to be nil")
	}
	if raw[1].Subject != nil {
		t.Errorf("expected subject to be nil")
	}

	// Verify the backing arrays are zeroed
	for i, b := range origSubj1 {
		if b != 0 {
			t.Errorf("expected byte %d of original subject 1 to be zeroed, got %d", i, b)
		}
	}
	for i, b := range origSubj2 {
		if b != 0 {
			t.Errorf("expected byte %d of original subject 2 to be zeroed, got %d", i, b)
		}
	}
}
