package model

type Dictionary struct {
	Version string `json:"version"`
	// UpdatedAt is display metadata only (e.g. for `proofboard sync` to show
	// "Dictionary updated to version X on <date>") — never used in any
	// detection/classification logic. Previously sent by the backend and
	// silently dropped since this struct had no matching field.
	UpdatedAt  string             `json:"updatedAt,omitempty"`
	Categories map[string]Signals `json:"categories"`
	// FeatureKeywords is a separate, flat vocabulary of generic, resume-style
	// feature nouns ("dashboard", "checkout", "onboarding"), not another
	// tier of the 25-category taxonomy. A commit's PrimaryCategory answers
	// "what kind of work" (Frontend & UI, API & Backend Services, ...); a
	// FeatureKeyword answers "which feature" (dashboard, checkout), giving
	// the AI outcome-summary prompt something concrete to write about instead
	// of two categories that are both still vague. Every entry is a generic
	// term common across SaaS/web/mobile products, never anything specific
	// to one company or codebase.
	FeatureKeywords []string `json:"featureKeywords,omitempty"`
	// StackSignals maps a manifest dependency name (e.g. "mongoose", "stripe")
	// to a human-facing framework/library label (e.g. "Mongoose", "Stripe"),
	// driving DetectStack's tech-stack detection. Downloaded taxonomy like
	// Categories/FeatureKeywords above, so recognizing a newly popular
	// library is a dictionary version bump, not a CLI release.
	StackSignals map[string]string `json:"stackSignals,omitempty"`
	// ManifestStackSignals extends StackSignals beyond the npm/package.json
	// ecosystem, keyed by manifest identifier (an exact basename, e.g.
	// "go.mod", "Cargo.toml", "requirements.txt"; or a "*.ext" suffix
	// pattern, e.g. "*.csproj", for manifests whose basename varies per
	// project) to that manifest's own dependency-substring → label table.
	// Lets Python/Go/Rust/PHP/Ruby/Java/.NET/Flutter/iOS/Elixir projects get
	// the same tech-stack detection breadth the npm ecosystem already has,
	// without hardcoding vocabulary into the CLI binary.
	ManifestStackSignals map[string]map[string]string `json:"manifestStackSignals,omitempty"`
	// IndustrySignals maps a manifest dependency name to an industry label
	// (e.g. "stripe" -> "Fintech", "twilio" -> "Telecom"), matched only
	// against dependency names already visible pre-shredding, never
	// org/repo/company names, so this stays NDA-safe by the same boundary as
	// Categories/StackSignals.
	IndustrySignals map[string]string `json:"industrySignals,omitempty"`
	// IndustryPathKeywords maps an industry label to generic module/folder-
	// name nouns (e.g. "vendors", "orders", "delivery") safe to substring-
	// match against TRACKED FILE PATHS, matched entirely on-device; only
	// the resolved label ever leaves the machine, never the raw path (same
	// NDA-safe boundary as IndustrySubjectKeywords). Often the strongest
	// industry signal available: a repo's own module names describe its
	// business domain more precisely than a payment-processor dependency
	// ever could (a marketplace, a SaaS tool, and a logistics platform can
	// all use the same payment SDK).
	IndustryPathKeywords map[string][]string `json:"industryPathKeywords,omitempty"`
	// IndustrySubjectKeywords maps an industry label to a list of generic,
	// human-reviewed phrases safe to substring-match against commit SUBJECT
	// TEXT (as opposed to IndustrySignals, which matches manifest dependency
	// names only), resolved on-device, before Phase 2's shredding, and only
	// the resolved label (never the matched phrase or subject text) is ever
	// transmitted.
	IndustrySubjectKeywords map[string][]string `json:"industrySubjectKeywords,omitempty"`
}

type Signals struct {
	Keywords []string `json:"keywords"`
	Paths    []string `json:"paths"`
	Impact   string   `json:"impact"`
}
