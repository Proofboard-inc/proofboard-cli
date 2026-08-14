package model

// StackReport is the locally-detected tech stack + structural signals sent
// optionally at link and sync time. Everything here is a label, count, or
// boolean; no file contents ever leave the machine.
type StackReport struct {
	// Languages maps a detected language name (e.g. "Go", "TypeScript") to
	// the number of tracked files with a matching extension.
	Languages map[string]int `json:"languages,omitempty"`
	// TechStack is the sorted list of detected framework/tool labels (e.g.
	// "React", "NestJS", "Prisma"), derived from dependency manifests.
	TechStack []string `json:"techStack,omitempty"`
	HasCI     bool     `json:"hasCI,omitempty"`
	HasTests  bool     `json:"hasTests,omitempty"`
	HasDocker bool     `json:"hasDocker,omitempty"`
	HasIaC    bool     `json:"hasIaC,omitempty"`
	// IndustryHints is a best-effort list of industry labels (e.g.
	// ["Fintech"], or ["E-commerce", "Logistics"] when a project genuinely
	// spans more than one), matched against the dictionary's IndustrySignals
	// (manifest dependency names) and IndustrySubjectKeywords (commit
	// subjects); generic hints, not a hard classification. Strongest match
	// first. Empty when nothing matched or no dictionary is available yet.
	IndustryHints []string `json:"industryHints,omitempty"`
}
