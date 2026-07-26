package phase7

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestUnsignedCanonicalPayloadOmitsDeviceSignature(t *testing.T) {
	payload := model.SyncPayload{DeviceKeyID: "device-key"}
	unsigned, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal unsigned payload: %v", err)
	}
	if bytes.Contains(unsigned, []byte(`"deviceSignature"`)) {
		t.Fatalf("unsigned canonical payload includes deviceSignature: %s", unsigned)
	}
	payload.DeviceSignature = "signed"
	signed, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal signed payload: %v", err)
	}
	if !bytes.Contains(signed, []byte(`"deviceSignature":"signed"`)) {
		t.Fatalf("transmitted payload omitted deviceSignature: %s", signed)
	}
}

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
	if !payload.AntiFraudSignals.LowCommitCount {
		t.Fatal("two commits must set lowCommitCount")
	}
	if payload.CapturedAt != now.Add(time.Minute).Format(time.RFC3339) {
		t.Fatalf("capturedAt = %q, want last commit time", payload.CapturedAt)
	}
}

func TestAssembleEnforcesEndpointLimits(t *testing.T) {
	commits := make([]model.SafeCommit, maxPayloadCommits+20)
	for i := range commits {
		commits[i] = model.SafeCommit{
			SHA:           "sha",
			TimestampUnix: int64(i + 1),
			Category:      "Feature Development",
			ImpactType:    "feature",
		}
	}
	clusters := make([]model.Cluster, maxPayloadClusters+5)
	payload := Assemble(AssemblyInput{Commits: commits, Clusters: clusters})
	if len(payload.SHAs) != maxPayloadCommits {
		t.Fatalf("commit count = %d, want %d", len(payload.SHAs), maxPayloadCommits)
	}
	if len(payload.MilestoneClusters) != maxPayloadClusters {
		t.Fatalf("cluster count = %d, want %d", len(payload.MilestoneClusters), maxPayloadClusters)
	}
	if payload.AntiFraudSignals.LowCommitCount {
		t.Fatal("large payload must not set lowCommitCount")
	}
}

func TestAssembleUsesPlainIdentityHashForMismatchSignal(t *testing.T) {
	payload := Assemble(AssemblyInput{
		Commits: []model.SafeCommit{{
			SHA:             "sha",
			AuthorEmailHash: "plain-sha256",
		}},
		EmailHash:         "per-project-hmac",
		IdentityEmailHash: "plain-sha256",
	})
	if payload.AntiFraudSignals.IdentityMismatch != 0 {
		t.Fatalf("identityMismatch = %d, want 0", payload.AntiFraudSignals.IdentityMismatch)
	}
}
