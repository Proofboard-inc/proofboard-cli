package phase5

import (
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestShredOutputContainsOnlySafeCommitFields(t *testing.T) {
	t.Parallel()
	raw := []model.RawCommit{{
		SHA:          "abc",
		Timestamp:    time.Unix(1, 0),
		Additions:    1,
		Deletions:    2,
		FilesChanged: 3,
		Subject:      []byte("secret internal project name"),
		FilePaths:    []string{"client/secret/path.go"},
		AuthorEmail:  "Dev@Example.com",
		Repository:   "private-repo",
		Organization: "private-org",
	}}
	signals := []model.CommitSignal{{
		SHA:             "abc",
		Timestamp:       time.Unix(1, 0),
		Additions:       1,
		Deletions:       2,
		FilesChanged:    3,
		PrimaryCategory: "API and Backend Services",
		ImpactType:      "feature",
	}}
	got := Shred(raw, signals)
	if got[0].SHA != "abc" {
		t.Fatalf("expected safe commit SHA to survive shredder")
	}
	if raw[0].Subject != nil || raw[0].FilePaths != nil || raw[0].AuthorEmail != "" || raw[0].Repository != "" || raw[0].Organization != "" {
		t.Fatalf("expected proprietary raw fields to be dropped")
	}
}

func TestShredWithNilSubject(t *testing.T) {
	t.Parallel()
	raw := []model.RawCommit{{
		SHA:          "abc",
		Timestamp:    time.Unix(1, 0),
		Additions:    1,
		Deletions:    2,
		FilesChanged: 3,
		Subject:      nil,
		FilePaths:    []string{"client/secret/path.go"},
		AuthorEmail:  "Dev@Example.com",
		Repository:   "private-repo",
		Organization: "private-org",
	}}
	signals := []model.CommitSignal{{
		SHA:             "abc",
		Timestamp:       time.Unix(1, 0),
		Additions:       1,
		Deletions:       2,
		FilesChanged:    3,
		PrimaryCategory: "API and Backend Services",
		ImpactType:      "feature",
	}}
	got := Shred(raw, signals)
	if got[0].SHA != "abc" {
		t.Fatalf("expected safe commit SHA to survive shredder")
	}
	if raw[0].Subject != nil || raw[0].FilePaths != nil || raw[0].AuthorEmail != "" || raw[0].Repository != "" || raw[0].Organization != "" {
		t.Fatalf("expected proprietary raw fields to be dropped")
	}
}

