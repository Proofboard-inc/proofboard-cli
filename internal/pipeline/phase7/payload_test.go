package phase7

import (
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestAssembleAddsTrustSignalsAndPreviousHead(t *testing.T) {
	now := time.Now().UTC()
	payload := Assemble(AssemblyInput{
		Commits: []model.SafeCommit{
			{SHA: "a", TimestampUnix: now.Unix(), Additions: 1, Deletions: 0, FilesChanged: 1, Category: "Feature Development", ImpactType: "feature", NoiseScore: 0.2, SignatureValid: true},
			{SHA: "b", TimestampUnix: now.Add(time.Minute).Unix(), Additions: 2, Deletions: 1, FilesChanged: 1, Category: "Bug Fixes & Maintenance", ImpactType: "bugfix", NoiseScore: 0.4, SignatureValid: false},
		},
		Clusters:          nil,
		OrgHash:           "org",
		RepoHash:          "repo",
		EmailHash:         "email",
		Provider:          "github",
		CLIVersion:        "1.0.0",
		DictionaryVersion: "1.0.0",
		PreviousHead:      "previous-head-sha",
	})

	if payload.PreviousHead != "previous-head-sha" {
		t.Fatalf("expected previous head to be retained, got %q", payload.PreviousHead)
	}
	if payload.AntiFraudSignals.SignedCommitRatio != 0.5 {
		t.Fatalf("expected signed commit ratio 0.5, got %v", payload.AntiFraudSignals.SignedCommitRatio)
	}
	if payload.ImpactScores.Feature != 0.5 || payload.ImpactScores.Bugfix != 0.5 {
		t.Fatalf("unexpected impact scores: %#v", payload.ImpactScores)
	}
}
