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

// Pure-Go, no-dependency stack detection — extension histograms and
// manifest text scans only, no go-enry/tree-sitter/CGO (decided scope: kept
// dependency-free, consistent with the CLI's fully pure-Go dependency graph
// and cross-compile requirements). Labels/counts only leave the machine —
// never file contents.

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

// defaultNpmFrameworkLabels is the fallback used only before the first
// dictionary fetch ever succeeds (fresh install, offline first run) — once a
// real dictionary is loaded, its StackSignals (server-driven, far larger,
// see cli-dictionary.ts) takes over entirely. Previously this ~15-entry map
// was the ONLY source, with no fallback bucket for anything outside it —
// which is why a NestJS+MongoDB+Stripe+Redis backend detected as just
// "Jest, NestJS": none of mongoose/stripe/ioredis/passport/aws-sdk were in
// this list, and unlisted dependencies were silently dropped, not merely
// deprioritized.
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

// stackSignalsOrDefault prefers the dictionary's server-driven StackSignals
// (kept current via dictionary version bumps, no CLI release needed) and
// only falls back to the small built-in table when no dictionary has been
// loaded yet.
func stackSignalsOrDefault(dict model.Dictionary) map[string]string {
	if len(dict.StackSignals) > 0 {
		return dict.StackSignals
	}
	return defaultNpmFrameworkLabels
}

// manifestFrameworkLabels is the small built-in fallback for every non-npm
// manifest, used only before the first dictionary fetch ever succeeds — same
// "before first fetch" contract as defaultNpmFrameworkLabels above. Once a
// real dictionary is loaded, its ManifestStackSignals (server-driven, far
// larger, see cli-dictionary.ts) takes over entirely per manifest. Keyed by
// manifest identifier — an exact basename, or a "*.ext" suffix pattern for
// manifests whose basename varies per project (see manifestKeyForFile).
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

// manifestExactKeys are manifest basenames matched exactly. Manifests whose
// basename varies per project (e.g. MyApp.csproj) are matched by suffix
// instead — see manifestKeyForFile.
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

// manifestKeyForFile maps an actual tracked file's basename to the manifest
// identifier used to key both the dictionary's ManifestStackSignals and the
// manifestFrameworkLabels fallback — "" if the file isn't a recognized
// manifest. package.json is handled separately (parsePackageJSONSignals,
// needs real JSON parsing to separate deps from devDeps).
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

// manifestSignalsOrDefault prefers the dictionary's server-driven
// ManifestStackSignals for a given manifest identifier and only falls back
// to the small built-in table for that identifier when no dictionary has
// been loaded yet — same pattern as stackSignalsOrDefault for the npm
// ecosystem.
func manifestSignalsOrDefault(manifestKey string, dict model.Dictionary) map[string]string {
	if table, ok := dict.ManifestStackSignals[manifestKey]; ok && len(table) > 0 {
		return table
	}
	return manifestFrameworkLabels[manifestKey]
}

// DetectStack inspects a repo's tracked files (via `git ls-files`) and
// returns language/framework labels plus structural flags. Best-effort: a
// detection failure (e.g. not a git repo) returns a zero-value report and an
// error the caller may safely ignore. dict supplies the server-driven
// stack/industry signal tables (see stackSignalsOrDefault) — pass the
// zero-value model.Dictionary{} to use the small built-in fallback only.
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

// Matches manifests by basename anywhere in the tree (still via the
// already-collected `git ls-files` list, no extra I/O), not just at repo
// root, so a monorepo (apps/frontend/package.json, apps/backend/go.mod, no
// manifest at the actual root) is detected. Capped at maxManifestsScanned to
// avoid a pathological monorepo making link/sync slow — raised from 20 to 30
// now that manifest coverage spans many more ecosystems (Python, Go, Rust,
// PHP, Ruby, Java, .NET, Flutter, iOS, Elixir), so a genuinely polyglot repo
// has more manifests competing for the same budget. Entirely best-effort: a
// parse failure on one manifest (handled inside parsePackageJSONSignals/
// scanTextManifestForFrameworks, which already return nil on read/parse
// error) never aborts detection for the others.
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
			if labels := manifestSignalsOrDefault(key, dict); len(labels) > 0 {
				addAll(scanTextManifestForFrameworks(filepath.Join(repoPath, f), labels))
			}
			scanned++
		}
	}

	// Folder/module names (e.g. src/modules/vendors, src/modules/delivery)
	// are matched against every tracked file, not just the manifest-scan
	// budget above — no I/O involved (already-collected path strings only),
	// and often the single strongest industry signal a repo has: its own
	// module names describe its business domain more precisely than a
	// payment-processor dependency ever could.
	countIndustryPathMatches(files, dict, industryCounts)

	for label := range found {
		report.TechStack = append(report.TechStack, label)
	}
	report.IndustryHints = topIndustries(industryCounts, maxIndustryHints)
}

// countIndustryPathMatches scans every tracked file's path (case-
// insensitive) for dict.IndustryPathKeywords matches and increments counts
// per matched industry label — at most once per file even if multiple
// keywords for the same industry match (e.g. a path containing both
// "orders" and "cart" only counts once toward E-commerce), so a single
// deeply-nested file can't dominate the ranking over genuinely distinct
// modules.
func countIndustryPathMatches(files []string, dict model.Dictionary, counts map[string]int) {
	if len(dict.IndustryPathKeywords) == 0 {
		return
	}
	for _, f := range files {
		lower := strings.ToLower(f)
		for label, keywords := range dict.IndustryPathKeywords {
			for _, kw := range keywords {
				if kw == "" {
					continue
				}
				if strings.Contains(lower, strings.ToLower(kw)) {
					counts[label]++
					break
				}
			}
		}
	}
}

// maxIndustryHints caps how many industry labels DetectStack/
// IndustryHintsFromCommits will ever report — a project can genuinely span
// more than one (e.g. an e-commerce backend that also handles logistics),
// but an unbounded list stops being a useful "hint" and starts looking like
// noise. Raised from 3 to 5: with manifest-dependency and path-keyword
// counts merged into one ranking, a real domain signal (e.g. path matches
// for "orders"/"delivery") could previously be crowded out of the top 3 by
// two unrelated single-dependency matches (e.g. a notification SDK, an
// analytics SDK) that had nothing to do with the project's actual business
// domain — 5 slots gives real multi-domain signals room to all surface.
const maxIndustryHints = 5

// topIndustries ranks industry labels by match count, most-frequent first,
// alphabetical tie-break for determinism, capped at max. A hint, not a hard
// classification. Empty slice if nothing matched.
func topIndustries(counts map[string]int, max int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sort.SliceStable(keys, func(i, j int) bool {
		return counts[keys[i]] > counts[keys[j]]
	})
	if len(keys) > max {
		keys = keys[:max]
	}
	return keys
}

// IndustryHintsFromCommits derives best-effort industry labels by matching
// commit subjects against dict.IndustrySubjectKeywords — pure substring
// containment against a fixed, server-curated phrase list, exactly like the
// existing feature-keyword matching in phase2. Never derives a label that
// isn't already one of the dictionary's pre-approved keys; never returns
// anything but those exact label strings. Must be called with raw commits
// whose Subject field is still populated (i.e. before Phase 2's shredding
// zeroes it) — this function does not mutate or retain the input.
//
// Fills in industries manifest-based detection (DetectStack / topIndustries
// above, matched against dependency names) couldn't see — e.g. a repo that
// talks to an industry-specific API over raw HTTP with no matching npm
// dependency. Only the resolved labels ever leave this function; the
// matched phrase and raw subject text never do.
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
	// Same alphabetical-tie-break, most-frequent-match ranking as the
	// manifest-based path above — one shared, deterministic policy for
	// ordering multiple matched industry labels.
	return topIndustries(counts, maxIndustryHints)
}

type packageJSONManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// parsePackageJSONSignals matches every dependency (regular + dev) against
// the given stack/industry signal tables. Only labels/industries — never the
// dependency list itself — are returned to the caller.
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

func scanTextManifestForFrameworks(path string, labels map[string]string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := strings.ToLower(string(data))
	var found []string
	for pkg, label := range labels {
		if strings.Contains(content, strings.ToLower(pkg)) {
			found = append(found, label)
		}
	}
	return found
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
