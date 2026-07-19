package phase7

import (
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

type AssemblyInput struct {
	Commits           []model.SafeCommit
	Clusters          []model.Cluster
	OrgHash           string
	RepoHash          string
	EmailHash         string
	Provider          string
	CLIVersion        string
	DictionaryVersion string
	ExpectedOrgHash   string
	PreviousHead      string
}

func Assemble(input AssemblyInput) model.SyncPayload {
	payload := model.SyncPayload{
		SHAs:              make([]string, 0, len(input.Commits)),
		Timestamps:        make([]int64, 0, len(input.Commits)),
		Additions:         make([]int, 0, len(input.Commits)),
		Deletions:         make([]int, 0, len(input.Commits)),
		FilesChanged:      make([]int, 0, len(input.Commits)),
		Categories:        make([]string, 0, len(input.Commits)),
		ImpactScores:      model.ImpactScores{},
		MilestoneClusters: input.Clusters,
		OrgHash:           input.OrgHash,
		RepoHash:          input.RepoHash,
		EmailHash:         input.EmailHash,
		Provider:          input.Provider,
		CapturedAt:        time.Now().UTC().Format(time.RFC3339),
		CLIVersion:        input.CLIVersion,
		DictionaryVersion: input.DictionaryVersion,
		PreviousHead:      input.PreviousHead,
		NotifyPush:        false,
		AntiFraudSignals: model.AntiFraudSignals{
			LowCommitCount:      len(input.Commits) < 5,
			OrgHashMismatch:     input.ExpectedOrgHash != "" && input.ExpectedOrgHash != input.OrgHash,
			SingleCommitRepoCap: false,
		},
	}

	impactCounts := make(map[string]int)
	totalNoise := 0.0
	identityMismatch := 0
	signedCount := 0

	for _, commit := range input.Commits {
		payload.SHAs = append(payload.SHAs, commit.SHA)
		payload.Timestamps = append(payload.Timestamps, commit.TimestampUnix)
		payload.Additions = append(payload.Additions, commit.Additions)
		payload.Deletions = append(payload.Deletions, commit.Deletions)
		payload.FilesChanged = append(payload.FilesChanged, commit.FilesChanged)
		cat := normalizeCategory(commit.Category)
		payload.Categories = append(payload.Categories, cat)
		impactCounts[normalizeImpact(commit.ImpactType)]++
		totalNoise += commit.NoiseScore
		if commit.SignatureValid {
			signedCount++
		}
		if input.EmailHash != "" && commit.AuthorEmailHash != "" && commit.AuthorEmailHash != input.EmailHash {
			identityMismatch++
		}
	}

	for i := range payload.MilestoneClusters {
		payload.MilestoneClusters[i].Category = normalizeCategory(payload.MilestoneClusters[i].Category)
		payload.MilestoneClusters[i].ImpactType = normalizeImpact(payload.MilestoneClusters[i].ImpactType)
		payload.MilestoneClusters[i].ImpactScale = normalizeScale(payload.MilestoneClusters[i].ImpactScale)
	}

	payload.AntiFraudSignals.IdentityMismatch = identityMismatch
	if len(input.Commits) > 0 {
		payload.AntiFraudSignals.SignedCommitRatio = float64(signedCount) / float64(len(input.Commits))
	}

	if len(input.Commits) > 0 {
		payload.AntiFraudSignals.AINoiseScore = totalNoise / float64(len(input.Commits))
		total := float64(len(input.Commits))
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

func normalizeCategory(cat string) string {
	switch strings.ToUpper(cat) {
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
		if cat == "Unclassified" {
			return "Feature Development"
		}
		return cat
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
