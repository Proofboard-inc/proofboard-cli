package phase7

import (
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

const (
	maxPayloadCommits = 5000
	// Phase4.Detect currently caps at 4 clusters per sync, so this is
	// dead code today — but if that cap is ever raised without updating this
	// constant in lockstep, this truncation would silently drop clusters (and
	// the commits inside them) from the transmitted payload, breaking the
	// Σ cluster.CommitCount == len(commits) invariant phase4 guarantees
	// internally (see TestDetectCommitCountInvariant in phase4).
	maxPayloadClusters = 50
)

type AssemblyInput struct {
	Commits           []model.SafeCommit
	Clusters          []model.Cluster
	OrgHash           string
	RepoHash          string
	EmailHash         string
	IdentityEmailHash string
	Provider          string
	CLIVersion        string
	DictionaryVersion string
	ExpectedOrgHash   string
	PreviousHead      string
	// Locally-detected tech stack + structural signals, optional.
	Stack *model.StackReport
	// IsDefaultBranch — see model.SyncPayload.IsDefaultBranch.
	IsDefaultBranch bool
}

func Assemble(input AssemblyInput) model.SyncPayload {
	commits := input.Commits
	if len(commits) > maxPayloadCommits {
		commits = commits[:maxPayloadCommits]
	}
	clusters := input.Clusters
	if len(clusters) > maxPayloadClusters {
		clusters = clusters[:maxPayloadClusters]
	}
	clusters = append([]model.Cluster(nil), clusters...)

	payload := model.SyncPayload{
		SHAs:              make([]string, 0, len(commits)),
		Timestamps:        make([]int64, 0, len(commits)),
		Additions:         make([]int, 0, len(commits)),
		Deletions:         make([]int, 0, len(commits)),
		FilesChanged:      make([]int, 0, len(commits)),
		Categories:        make([]string, 0, len(commits)),
		ImpactScores:      model.ImpactScores{},
		MilestoneClusters: clusters,
		OrgHash:           input.OrgHash,
		RepoHash:          input.RepoHash,
		EmailHash:         input.EmailHash,
		Provider:          input.Provider,
		CapturedAt:        time.Now().UTC().Format(time.RFC3339),
		CLIVersion:        input.CLIVersion,
		DictionaryVersion: input.DictionaryVersion,
		PreviousHead:      input.PreviousHead,
		NotifyPush:        false,
		Stack:             input.Stack,
		IsDefaultBranch:   input.IsDefaultBranch,
		AntiFraudSignals: model.AntiFraudSignals{
			LowCommitCount:      len(commits) < 3,
			OrgHashMismatch:     input.ExpectedOrgHash != "" && input.ExpectedOrgHash != input.OrgHash,
			SingleCommitRepoCap: false,
		},
	}

	impactCounts := make(map[string]int)
	totalNoise := 0.0
	identityMismatch := 0
	signedCount := 0
	identityEmailHash := input.IdentityEmailHash
	if identityEmailHash == "" {
		identityEmailHash = input.EmailHash
	}

	var capturedAt int64
	for _, commit := range commits {
		payload.SHAs = append(payload.SHAs, commit.SHA)
		payload.Timestamps = append(payload.Timestamps, commit.TimestampUnix)
		payload.Additions = append(payload.Additions, commit.Additions)
		payload.Deletions = append(payload.Deletions, commit.Deletions)
		payload.FilesChanged = append(payload.FilesChanged, commit.FilesChanged)
		cat := normalizeCategory(commit.Category)
		payload.Categories = append(payload.Categories, cat)
		if commit.TimestampUnix > capturedAt {
			capturedAt = commit.TimestampUnix
		}
		impactCounts[normalizeImpact(commit.ImpactType)]++
		totalNoise += commit.NoiseScore
		if commit.SignatureValid {
			signedCount++
		}
		if identityEmailHash != "" && commit.AuthorEmailHash != "" && commit.AuthorEmailHash != identityEmailHash {
			identityMismatch++
		}
	}

	for i := range payload.MilestoneClusters {
		payload.MilestoneClusters[i].Category = normalizeCategory(payload.MilestoneClusters[i].Category)
		payload.MilestoneClusters[i].ImpactType = normalizeImpact(payload.MilestoneClusters[i].ImpactType)
		payload.MilestoneClusters[i].ImpactScale = normalizeScale(payload.MilestoneClusters[i].ImpactScale)
	}

	payload.AntiFraudSignals.IdentityMismatch = identityMismatch
	if capturedAt > 0 {
		payload.CapturedAt = time.Unix(capturedAt, 0).UTC().Format(time.RFC3339)
	}
	if len(commits) > 0 {
		payload.AntiFraudSignals.SignedCommitRatio = float64(signedCount) / float64(len(commits))
	}

	if len(commits) > 0 {
		payload.AntiFraudSignals.AINoiseScore = totalNoise / float64(len(commits))
		total := float64(len(commits))
		payload.ImpactScores.Feature = float64(impactCounts["feature"]) / total
		payload.ImpactScores.Bugfix = float64(impactCounts["bugfix"]) / total
		payload.ImpactScores.Refactor = float64(impactCounts["refactor"]) / total
		payload.ImpactScores.Ship = float64(impactCounts["ship"]) / total
		payload.ImpactScores.Maintenance = float64(impactCounts["maintenance"]) / total
	} else {
		payload.ImpactScores = model.ImpactScores{}
	}

	return payload
}

// canonicalCategories must stay in sync with the dictionary's category keys
// (internal/dictionary/dictionary.json) and the backend's
// CLI_CATEGORY_VOCABULARY (cli-dictionary.ts) — these are the only values
// the backend accepts without a 400.
var canonicalCategories = []string{
	"Authentication & Security",
	"API & Backend Services",
	"Database & Data Layer",
	"Infrastructure & DevOps",
	"Performance & Optimisation",
	"Frontend & UI",
	"Design System & Components",
	"Landing Pages & Marketing",
	"Mobile Development",
	"Real-time & WebSockets",
	"Search & Discovery",
	"Analytics & Observability",
	"Data Migration & ETL",
	"Payments & Billing",
	"Email & Notifications",
	"Access Control & Permissions",
	"AI & Machine Learning",
	"Internationalisation & Localisation",
	"Testing & QA",
	"Documentation",
	"Bug Fixes & Maintenance",
	"Refactoring",
	"File Storage & Media",
	"Developer Tooling",
	"Feature Development",
}

var canonicalCategoryByUpper = func() map[string]string {
	m := make(map[string]string, len(canonicalCategories))
	for _, c := range canonicalCategories {
		m[strings.ToUpper(c)] = c
	}
	return m
}()

func normalizeCategory(cat string) string {
	upper := strings.ToUpper(strings.TrimSpace(cat))

	// The classifier (phase2/intent.go, via
	// phase5's shredder) already produces the full canonical category name —
	// e.g. "Authentication & Security" — not a short code. The switch below
	// only ever matched short aliases like "AUTH", so every real
	// classification result was silently falling through to the
	// "Feature Development" default before this exact-match lookup was
	// added, discarding all of phase2's classification work regardless of
	// how detailed the dictionary is. Check the full canonical form first.
	if canonical, ok := canonicalCategoryByUpper[upper]; ok {
		return canonical
	}

	switch upper {
	case "AUTH", "AUTHENTICATION":
		return "Authentication & Security"
	case "FRONTEND", "UI":
		return "Frontend & UI"
	case "API", "BACKEND":
		return "API & Backend Services"
	case "DATABASE", "DB":
		return "Database & Data Layer"
	case "INFRASTRUCTURE", "DEVOPS":
		return "Infrastructure & DevOps"
	case "PERFORMANCE", "OPTIMISATION":
		return "Performance & Optimisation"
	case "PAYMENTS", "BILLING":
		return "Payments & Billing"
	case "TESTING", "QA":
		return "Testing & QA"
	case "DOCUMENTATION", "DOCS":
		return "Documentation"
	case "BUG", "BUGFIX", "FIX":
		return "Bug Fixes & Maintenance"
	case "FEATURE", "UNCLASSIFIED":
		return "Feature Development"
	case "REFACTOR", "REFACTORING":
		return "Refactoring"
	case "MOBILE":
		return "Mobile Development"
	case "AI", "MACHINE LEARNING":
		return "AI & Machine Learning"
	case "TOOLING", "DEVELOPER TOOLING":
		return "Developer Tooling"
	default:
		// Never pass an unmapped/unknown category through raw — the
		// backend's CLI_CATEGORY_VOCABULARY rejects anything outside its
		// ~25-value list with a 400, and an unmapped category here previously
		// slipped straight through unnormalized. "Feature Development" is the
		// same safe catch-all already used for the (dead, since ToUpper makes
		// it match the FEATURE/UNCLASSIFIED case above) "Unclassified" check
		// this replaces.
		return "Feature Development"
	}
}

func normalizeImpact(impact string) string {
	switch strings.ToLower(impact) {
	case "feature":
		return "feature"
	case "bugfix", "fix":
		return "bugfix"
	case "refactor":
		return "refactor"
	case "ship":
		return "ship"
	case "maintenance":
		return "maintenance"
	default:
		return "feature"
	}
}

func normalizeScale(scale string) string {
	switch strings.ToLower(scale) {
	case "small":
		return "small"
	case "medium":
		return "medium"
	case "large":
		return "large"
	default:
		return "small"
	}
}
