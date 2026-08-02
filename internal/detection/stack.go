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

var npmFrameworkLabels = map[string]string{
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

var goModFrameworkLabels = map[string]string{
	"github.com/gin-gonic/gin": "Gin",
	"github.com/labstack/echo": "Echo",
	"github.com/gofiber/fiber": "Fiber",
	"github.com/spf13/cobra":   "Cobra",
}

var textManifestFrameworkLabels = map[string]map[string]string{
	"requirements.txt": {"django": "Django", "flask": "Flask", "fastapi": "FastAPI"},
	"pyproject.toml":   {"django": "Django", "flask": "Flask", "fastapi": "FastAPI"},
	"Cargo.toml":       {"actix-web": "Actix Web", "rocket": "Rocket", "axum": "Axum"},
	"Gemfile":          {"rails": "Ruby on Rails", "sinatra": "Sinatra"},
	"composer.json":    {"laravel/framework": "Laravel", "symfony/symfony": "Symfony"},
}

// DetectStack inspects a repo's tracked files (via `git ls-files`) and
// returns language/framework labels plus structural flags. Best-effort: a
// detection failure (e.g. not a git repo) returns a zero-value report and an
// error the caller may safely ignore.
func DetectStack(repoPath string) (model.StackReport, error) {
	report := model.StackReport{Languages: map[string]int{}}

	files, err := trackedFiles(repoPath)
	if err != nil {
		return report, err
	}

	detectLanguages(&report, files)
	detectFrameworks(&report, repoPath, files)
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
// avoid a pathological monorepo making link/sync slow. Entirely best-effort:
// a parse failure on one manifest (handled inside parsePackageJSONFrameworks/
// scanTextManifestForFrameworks, which already return nil on read/parse
// error) never aborts detection for the others.
const maxManifestsScanned = 20

func detectFrameworks(report *model.StackReport, repoPath string, files []string) {
	found := make(map[string]bool)
	addAll := func(labels []string) {
		for _, l := range labels {
			found[l] = true
		}
	}

	scanned := 0
	for _, f := range files {
		if scanned >= maxManifestsScanned {
			break
		}
		base := filepath.Base(f)
		switch base {
		case "package.json":
			addAll(parsePackageJSONFrameworks(filepath.Join(repoPath, f)))
			scanned++
		case "go.mod":
			addAll(scanTextManifestForFrameworks(filepath.Join(repoPath, f), goModFrameworkLabels))
			scanned++
		default:
			if labels, ok := textManifestFrameworkLabels[base]; ok {
				addAll(scanTextManifestForFrameworks(filepath.Join(repoPath, f), labels))
				scanned++
			}
		}
	}

	for label := range found {
		report.TechStack = append(report.TechStack, label)
	}
}

type packageJSONManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func parsePackageJSONFrameworks(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg packageJSONManifest
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	var found []string
	for name := range pkg.Dependencies {
		if label, ok := npmFrameworkLabels[name]; ok {
			found = append(found, label)
		}
	}
	for name := range pkg.DevDependencies {
		if label, ok := npmFrameworkLabels[name]; ok {
			found = append(found, label)
		}
	}
	return found
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
