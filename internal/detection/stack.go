package detection

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/proofboard/proofboard/internal/model"
)

var extensionToLanguage = map[string]string{
	".go":    "Go",
	".ts":    "TypeScript",
	".tsx":   "TypeScript",
	".js":    "JavaScript",
	".jsx":   "JavaScript",
	".py":    "Python",
	".rb":    "Ruby",
	".java":  "Java",
	".rs":    "Rust",
	".php":   "PHP",
	".c":     "C",
	".cpp":   "C++",
	".cc":    "C++",
	".cs":    "C#",
	".swift": "Swift",
	".kt":    "Kotlin",
	".m":     "Objective-C",
	".scala": "Scala",
	".sh":    "Shell",
	".html":  "HTML",
	".css":   "CSS",
	".scss":  "SCSS",
	".vue":   "Vue",
}

var defaultNpmFrameworkLabels = map[string]string{
	"react":            "React",
	"next":             "Next.js",
	"vue":              "Vue",
	"@angular/core":    "Angular",
	"svelte":           "Svelte",
	"express":          "Express",
	"@nestjs/core":     "NestJS",
	"fastify":          "Fastify",
	"tailwindcss":      "Tailwind CSS",
	"prisma":           "Prisma",
	"typeorm":          "TypeORM",
	"jest":             "Jest",
	"vitest":           "Vitest",
	"cypress":          "Cypress",
	"@playwright/test": "Playwright",
}

func stackSignalsOrDefault(dict model.Dictionary) map[string]string {
	if len(dict.StackSignals) > 0 {
		return dict.StackSignals
	}
	return defaultNpmFrameworkLabels
}

var manifestFrameworkLabels = map[string]map[string]string{
	"go.mod": {
		"github.com/gin-gonic/gin": "Gin",
		"github.com/labstack/echo": "Echo",
		"github.com/gofiber/fiber": "Fiber",
		"github.com/spf13/cobra":   "Cobra",
	},
	"requirements.txt": {"django": "Django", "flask": "Flask", "fastapi": "FastAPI"},
	"pyproject.toml":   {"django": "Django", "flask": "Flask", "fastapi": "FastAPI"},
	"Cargo.toml":       {"actix-web": "Actix Web", "rocket": "Rocket", "axum": "Axum"},
	"Gemfile":          {"rails": "Ruby on Rails", "sinatra": "Sinatra"},
	"composer.json":    {"laravel/framework": "Laravel", "symfony/symfony": "Symfony"},
}

var manifestExactKeys = map[string]bool{
	"go.mod":           true,
	"requirements.txt": true,
	"pyproject.toml":   true,
	"Pipfile":          true,
	"Cargo.toml":       true,
	"Gemfile":          true,
	"composer.json":    true,
	"pom.xml":          true,
	"build.gradle":     true,
	"build.gradle.kts": true,
	"pubspec.yaml":     true,
	"Podfile":          true,
	"mix.exs":          true,
}

func manifestKeyForFile(base string) string {
	if manifestExactKeys[base] {
		return base
	}
	switch {
	case strings.HasSuffix(base, ".csproj"):
		return "*.csproj"
	case strings.HasSuffix(base, ".fsproj"):
		return "*.fsproj"
	}
	return ""
}

func manifestSignalsOrDefault(manifestKey string, dict model.Dictionary) map[string]string {
	if table, ok := dict.ManifestStackSignals[manifestKey]; ok && len(table) > 0 {
		return table
	}
	return manifestFrameworkLabels[manifestKey]
}

func DetectStack(repoPath string, dict model.Dictionary) (model.StackReport, error) {
	report := model.StackReport{Languages: map[string]int{}}

	files, err := trackedFiles(repoPath)
	if err != nil {
		return report, err
	}

	detectLanguages(&report, files)
	detectFrameworks(&report, repoPath, files, dict)
	detectStructuralFlags(&report, files)

	sort.Strings(report.TechStack)
	return report, nil
}

func trackedFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "ls-files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func detectLanguages(report *model.StackReport, files []string) {
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if lang, ok := extensionToLanguage[ext]; ok {
			report.Languages[lang]++
		}
	}
}

const maxManifestsScanned = 30

func detectFrameworks(report *model.StackReport, repoPath string, files []string, dict model.Dictionary) {
	found := make(map[string]bool)
	addAll := func(labels []string) {
		for _, l := range labels {
			found[l] = true
		}
	}
	stackSignals := stackSignalsOrDefault(dict)
	industryCounts := make(map[string]int)

	scanned := 0
	for _, f := range files {
		if scanned >= maxManifestsScanned {
			break
		}
		base := filepath.Base(f)
		if base == "package.json" {
			labels, industries := parsePackageJSONSignals(
				filepath.Join(repoPath, f), stackSignals, dict.IndustrySignals,
			)
			addAll(labels)
			for _, ind := range industries {
				industryCounts[ind]++
			}
			scanned++
			continue
		}
		if key := manifestKeyForFile(base); key != "" {
			labels := manifestSignalsOrDefault(key, dict)
			// Non-npm manifests (go.mod, requirements.txt, Cargo.toml, etc.)
			// now resolve industries the same way package.json always did —
			// dict.IndustrySignals is matched by raw substring against the
			// manifest text regardless of ecosystem, so a Go CLI's go.mod
			// dependency on cobra/urfave-cli, or a Rust CLI's Cargo.toml
			// dependency on clap, can now surface a "Developer Tools &
			// Infrastructure" hint the same way an npm project's package.json
			// already could for its own industry signals. Previously this
			// code path only ever resolved framework labels — a Go CLI tool
			// (like this CLI's own repo) had literally no way to contribute
			// an industry signal from its manifest at all.
			if len(labels) > 0 || len(dict.IndustrySignals) > 0 {
				fwLabels, industries := scanTextManifestForFrameworks(
					filepath.Join(repoPath, f), labels, dict.IndustrySignals,
				)
				addAll(fwLabels)
				for _, ind := range industries {
					industryCounts[ind]++
				}
			}
			scanned++
		}
	}

	countIndustryPathMatches(files, dict, industryCounts)

	for label := range found {
		report.TechStack = append(report.TechStack, label)
	}
	report.IndustryHints = topIndustriesConfident(industryCounts, MaxIndustryHints)
}

// countIndustryPathMatches scans every tracked file's path for
// dict.IndustryPathKeywords matches, counting DISTINCT containing
// directories per label (immediate parent dir, not top-level segment —
// top-level-only dedup breaks on repos where most files share one root,
// e.g. Next.js app router under app/).
func countIndustryPathMatches(files []string, dict model.Dictionary, counts map[string]int) {
	if len(dict.IndustryPathKeywords) == 0 {
		return
	}
	seenDirs := make(map[string]map[string]bool)
	for _, f := range files {
		lower := strings.ToLower(f)
		dir := ""
		if idx := strings.LastIndex(lower, "/"); idx >= 0 {
			dir = lower[:idx]
		}
		for label, keywords := range dict.IndustryPathKeywords {
			for _, kw := range keywords {
				if kw == "" {
					continue
				}
				if strings.Contains(lower, strings.ToLower(kw)) {
					if seenDirs[label] == nil {
						seenDirs[label] = make(map[string]bool)
					}
					seenDirs[label][dir] = true
					break
				}
			}
		}
	}
	for label, dirs := range seenDirs {
		counts[label] += len(dirs)
	}
}

// MaxIndustryHints caps how many industry labels DetectStack/IndustryHintsFromCommits
// will report at once; exported so callers outside this package (sync.go's
// subject-hint merge) can share the same cap instead of hardcoding their own.
const MaxIndustryHints = 5
const minIndustryMatches = 2

func topIndustriesConfident(counts map[string]int, max int) []string {
	type entry struct {
		label string
		count int
	}
	var entries []entry
	for k, c := range counts {
		if c >= minIndustryMatches {
			entries = append(entries, entry{k, c})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].label < entries[j].label
	})
	if len(entries) > max {
		entries = entries[:max]
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.label
	}
	return out
}

func IndustryHintsFromCommits(raw []model.RawCommit, dict model.Dictionary) []string {
	if len(dict.IndustrySubjectKeywords) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, commit := range raw {
		if len(commit.Subject) == 0 {
			continue
		}
		subjectLower := strings.ToLower(string(commit.Subject))
		for label, phrases := range dict.IndustrySubjectKeywords {
			for _, phrase := range phrases {
				if phrase == "" {
					continue
				}
				if strings.Contains(subjectLower, strings.ToLower(phrase)) {
					counts[label]++
					break
				}
			}
		}
	}
	return topIndustriesConfident(counts, MaxIndustryHints)
}

type packageJSONManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func parsePackageJSONSignals(
	path string, stackSignals, industrySignals map[string]string,
) (frameworks []string, industries []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var pkg packageJSONManifest
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil
	}
	check := func(name string) {
		if label, ok := stackSignals[name]; ok {
			frameworks = append(frameworks, label)
		}
		if industry, ok := industrySignals[name]; ok {
			industries = append(industries, industry)
		}
	}
	for name := range pkg.Dependencies {
		check(name)
	}
	for name := range pkg.DevDependencies {
		check(name)
	}
	return frameworks, industries
}

// scanTextManifestForFrameworks now also resolves industries, mirroring
// parsePackageJSONSignals — see the call site comment in detectFrameworks
// for why this matters for non-npm (Go/Rust/Python/etc.) manifests.
func scanTextManifestForFrameworks(
	path string, labels, industrySignals map[string]string,
) (frameworks []string, industries []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	content := strings.ToLower(string(data))
	for pkg, label := range labels {
		if strings.Contains(content, strings.ToLower(pkg)) {
			frameworks = append(frameworks, label)
		}
	}
	for pkg, industry := range industrySignals {
		if strings.Contains(content, strings.ToLower(pkg)) {
			industries = append(industries, industry)
		}
	}
	return frameworks, industries
}

func detectStructuralFlags(report *model.StackReport, files []string) {
	for _, f := range files {
		lower := strings.ToLower(f)
		base := filepath.Base(lower)

		if strings.HasPrefix(lower, ".github/workflows/") ||
			lower == ".gitlab-ci.yml" ||
			lower == "azure-pipelines.yml" ||
			strings.HasPrefix(lower, ".circleci/") ||
			base == "jenkinsfile" {
			report.HasCI = true
		}

		if base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") || strings.Contains(lower, "docker-compose") {
			report.HasDocker = true
		}

		if strings.HasSuffix(lower, "_test.go") ||
			strings.Contains(lower, ".spec.") ||
			strings.Contains(lower, ".test.") ||
			strings.Contains(lower, "__tests__/") ||
			strings.Contains(lower, "/test/") ||
			strings.Contains(lower, "/tests/") {
			report.HasTests = true
		}

		if strings.HasSuffix(lower, ".tf") ||
			strings.Contains(lower, "/helm/") ||
			strings.Contains(lower, "/k8s/") ||
			strings.Contains(lower, "kustomization.yaml") {
			report.HasIaC = true
		}
	}
}
