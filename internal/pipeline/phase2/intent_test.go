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

// A commit whose Subject alone gives no signal ("updates") must still
// classify correctly using Body text — the biggest accuracy lever. Also
// verifies Body is nil/zeroed after Classify returns, mirroring Subject.
func TestClassifySubjectAloneInsufficientBodyClassifies(t *testing.T) {
	dict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Payments": {
				Keywords: []string{"stripe", "payment"},
				Impact:   "feature",
			},
			"Documentation": {
				Keywords: []string{"docs"},
				Impact:   "docs",
			},
		},
	}

	raw := []model.RawCommit{
		{
			SHA:       "sha1",
			Timestamp: time.Now(),
			Subject:   []byte("updates"),
			Body:      []byte("adds chargeCard() and imports stripe"),
			FilePaths: []string{"internal/payments/charge.go"},
		},
	}
	origBody := raw[0].Body

	signals := Classify(raw, dict)

	if signals[0].PrimaryCategory != "Payments" {
		t.Fatalf("expected Payments category from body signal, got %s", signals[0].PrimaryCategory)
	}
	if raw[0].Body != nil {
		t.Fatalf("expected Body to be nil after Classify")
	}
	for i, b := range origBody {
		if b != 0 {
			t.Errorf("expected byte %d of original body to be zeroed, got %d", i, b)
		}
	}
}

// SortedCategoryNames must return a stable, alphabetically-sorted
// order regardless of map insertion/iteration order, so Classify's tie-break
// is deterministic.
func TestSortedCategoryNames(t *testing.T) {
	categories := map[string]model.Signals{
		"Zebra": {},
		"Alpha": {},
		"Mango": {},
	}
	want := []string{"Alpha", "Mango", "Zebra"}

	for i := 0; i < 20; i++ {
		got := sortedCategoryNames(categories)
		if len(got) != len(want) {
			t.Fatalf("expected %d names, got %d", len(want), len(got))
		}
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("iteration %d: expected %v, got %v", i, want, got)
			}
		}
	}
}

// When two or more categories tie on score for the same commit,
// Classify must deterministically pick the same category every time (the
// alphabetically-first tied name), never varying run to run due to Go's
// randomized map iteration order.
func TestClassifyDeterministicTieBreak(t *testing.T) {
	dict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Zebra": {Keywords: []string{"widget"}, Impact: "feature"},
			"Alpha": {Keywords: []string{"widget"}, Impact: "feature"},
			"Mango": {Keywords: []string{"widget"}, Impact: "feature"},
		},
	}

	for i := 0; i < 50; i++ {
		raw := []model.RawCommit{
			{
				SHA:       "sha1",
				Timestamp: time.Now(),
				Subject:   []byte("widget update"),
			},
		}
		signals := Classify(raw, dict)
		if signals[0].PrimaryCategory != "Alpha" {
			t.Fatalf("iteration %d: expected deterministic tie-break to Alpha, got %s", i, signals[0].PrimaryCategory)
		}
	}
}

// A commit where two categories tie on total score must resolve to the one
// backed by a structural file-path match, even when it is alphabetically
// last — structural evidence (which files actually changed) is more
// trustworthy than a text-only keyword match, so it wins ties.
func TestClassifyStructuralPathBreaksScoreTies(t *testing.T) {
	dict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			// Alphabetically first, purely text-based: subject keyword (+2)
			// plus a body keyword (+1) = 3, no path.
			"Aaa Text Category": {
				Keywords: []string{"widget", "gadget"},
				Impact:   "feature",
			},
			// Alphabetically last, structural: one path match (+3), no keywords.
			"Zzz Structural Category": {
				Paths:  []string{"src/core/"},
				Impact: "feature",
			},
		},
	}

	raw := []model.RawCommit{
		{
			SHA:       "sha1",
			Timestamp: time.Now(),
			Subject:   []byte("widget change"),           // Aaa: +2
			Body:      []byte("also touches the gadget"), // Aaa: +1  => total 3
			FilePaths: []string{"src/core/engine.ts"},    // Zzz: +3  => total 3
		},
	}

	signals := Classify(raw, dict)
	if signals[0].PrimaryCategory != "Zzz Structural Category" {
		t.Fatalf("expected the path-backed category to win the tie, got %s", signals[0].PrimaryCategory)
	}
}

// A 20-file commit with 2 markdown files and 18 backend
// files must classify as the backend category, not Documentation — the
// structural (path) signal must not be hijacked by determinism-only changes.
// This does not test path-score normalization, only that the existing
// path-weighted signal still wins as expected once iteration is deterministic.
func TestClassifyLargeCommitNotHijackedByFewDocFiles(t *testing.T) {
	dict := model.Dictionary{
		Version: "1.0.0",
		Categories: map[string]model.Signals{
			"Documentation": {
				Keywords: []string{"docs"},
				Paths:    []string{".md", "docs/"},
				Impact:   "docs",
			},
			"Backend": {
				Keywords: []string{"api"},
				Paths:    []string{"internal/"},
				Impact:   "feature",
			},
		},
	}

	filePaths := make([]string, 0, 20)
	filePaths = append(filePaths, "docs/readme.md", "docs/changelog.md")
	for i := 0; i < 18; i++ {
		filePaths = append(filePaths, "internal/service/handler.go")
	}

	raw := []model.RawCommit{
		{
			SHA:          "sha1",
			Timestamp:    time.Now(),
			FilesChanged: len(filePaths),
			Subject:      []byte("update handlers and docs"),
			FilePaths:    filePaths,
		},
	}

	signals := Classify(raw, dict)
	if signals[0].PrimaryCategory != "Backend" {
		t.Fatalf("expected Backend category for a commit dominated by backend files, got %s", signals[0].PrimaryCategory)
	}
}

// Regression test for generic milestone titles ("I designed a medium-scale
// API to enhance backend services"): real commit subjects are usually too
// terse to contain a feature-keyword phrase ("fix null check" says nothing),
// but the module a commit actually touches reliably does. FeatureKeyword
// matching must also check FilePaths, not just subject/body text.
func TestClassifyFeatureKeywordMatchesFilePathWhenSubjectIsUninformative(t *testing.T) {
	dict := model.Dictionary{
		Version:         "1.0.0",
		Categories:      map[string]model.Signals{"API & Backend Services": {Impact: "feature"}},
		FeatureKeywords: []string{"vendors", "checkout"},
	}

	raw := []model.RawCommit{
		{
			SHA:          "sha1",
			Timestamp:    time.Now(),
			FilesChanged: 1,
			Subject:      []byte("fix null check"),
			FilePaths:    []string{"src/modules/vendors/vendors.service.ts"},
		},
	}

	signals := Classify(raw, dict)
	if signals[0].FeatureKeyword != "vendors" {
		t.Fatalf("FeatureKeyword = %q, want %q from file path match despite an uninformative subject", signals[0].FeatureKeyword, "vendors")
	}
}
