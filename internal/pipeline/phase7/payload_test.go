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

// Assemble must copy Stack through unchanged when present, and omit
// it (nil) when absent — plumbing only.
func TestAssemblePassesStackThrough(t *testing.T) {
	stack := &model.StackReport{Languages: map[string]int{"Go": 1}, HasTests: true}

	withStack := Assemble(AssemblyInput{Stack: stack})
	if withStack.Stack != stack {
		t.Fatalf("expected Stack to be passed through unchanged, got %#v", withStack.Stack)
	}

	withoutStack := Assemble(AssemblyInput{})
	if withoutStack.Stack != nil {
		t.Fatalf("expected nil Stack when AssemblyInput.Stack is omitted, got %#v", withoutStack.Stack)
	}
}

func TestAssemblePassesIsDefaultBranchThrough(t *testing.T) {
	onDefault := Assemble(AssemblyInput{IsDefaultBranch: true})
	if !onDefault.IsDefaultBranch {
		t.Fatalf("expected IsDefaultBranch=true to pass through unchanged")
	}

	onManual := Assemble(AssemblyInput{IsDefaultBranch: false})
	if onManual.IsDefaultBranch {
		t.Fatalf("expected IsDefaultBranch=false to pass through unchanged")
	}
}

// NormalizeCategory must map every known alias correctly (regression)
// and must never pass an unmapped/unknown category through raw — it must
// always fall back to "Feature Development" so the backend's ~25-value
// CLI_CATEGORY_VOCABULARY @IsIn validator never 400s on an unrecognized
// category string.
func TestNormalizeCategory(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"known alias auth", "auth", "Authentication & Security"},
		{"known alias frontend", "UI", "Frontend & UI"},
		{"known alias documentation", "docs", "Documentation"},
		{"known alias bugfix", "fix", "Bug Fixes & Maintenance"},
		{"exact Unclassified falls through FEATURE case", "Unclassified", "Feature Development"},
		{"unknown category never passes through raw", "Some Future Category", "Feature Development"},
		{"empty string never passes through raw", "", "Feature Development"},
		{"arbitrary garbage never passes through raw", "XYZ", "Feature Development"},
		// FIX: these are what phase2/intent.go's Classify() (via phase5's
		// shredder) actually produces — the full canonical category name, not
		// a short alias. Before the canonicalCategoryByUpper lookup was added,
		// every one of these fell through to the default case and got
		// silently relabeled "Feature Development", discarding all real
		// classification. This is the regression test for that bug.
		{"full canonical name passes through: Authentication & Security", "Authentication & Security", "Authentication & Security"},
		{"full canonical name passes through: API & Backend Services", "API & Backend Services", "API & Backend Services"},
		{"full canonical name passes through: Design System & Components", "Design System & Components", "Design System & Components"},
		{"full canonical name passes through: Real-time & WebSockets", "Real-time & WebSockets", "Real-time & WebSockets"},
		{"full canonical name passes through: Access Control & Permissions", "Access Control & Permissions", "Access Control & Permissions"},
		{"full canonical name is case-insensitive", "authentication & security", "Authentication & Security"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCategory(tc.in)
			if got != tc.want {
				t.Errorf("normalizeCategory(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
