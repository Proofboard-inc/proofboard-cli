package detection

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("add", "-A")
	run("commit", "-m", "fixture commit")
}

func TestDetectStackNodeReactRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "package.json", `{
		"dependencies": { "react": "^18.0.0", "next": "^14.0.0" },
		"devDependencies": { "jest": "^29.0.0" }
	}`)
	writeFile(t, dir, "src/App.tsx", "export default function App() { return null }")
	writeFile(t, dir, "src/App.test.ts", "test('renders', () => {})")
	writeFile(t, dir, ".github/workflows/ci.yml", "name: CI\non: [push]")
	commitAll(t, dir)

	report, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}

	if report.Languages["TypeScript"] != 2 {
		t.Errorf("Languages[TypeScript] = %d, want 2", report.Languages["TypeScript"])
	}
	if !contains(report.TechStack, "React") || !contains(report.TechStack, "Next.js") || !contains(report.TechStack, "Jest") {
		t.Errorf("TechStack = %v, want React, Next.js, Jest", report.TechStack)
	}
	if !report.HasCI {
		t.Error("expected HasCI = true")
	}
	if !report.HasTests {
		t.Error("expected HasTests = true")
	}
	if report.HasDocker {
		t.Error("expected HasDocker = false")
	}
}

func TestDetectStackGoRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "go.mod", "module example.com/svc\n\nrequire github.com/gin-gonic/gin v1.9.0\n")
	writeFile(t, dir, "main.go", "package main\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\nfunc TestMain(t *testing.T) {}\n")
	writeFile(t, dir, "Dockerfile", "FROM golang:1.22\n")
	commitAll(t, dir)

	report, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}

	if report.Languages["Go"] != 2 {
		t.Errorf("Languages[Go] = %d, want 2", report.Languages["Go"])
	}
	if !contains(report.TechStack, "Gin") {
		t.Errorf("TechStack = %v, want Gin", report.TechStack)
	}
	if !report.HasDocker {
		t.Error("expected HasDocker = true")
	}
	if !report.HasTests {
		t.Error("expected HasTests = true")
	}
	if report.HasCI {
		t.Error("expected HasCI = false")
	}
}

func TestDetectStackEmptyRepoReturnsZeroValueNoError(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "README.md", "# Nothing here\n")
	commitAll(t, dir)

	report, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if len(report.Languages) != 0 {
		t.Errorf("Languages = %v, want empty", report.Languages)
	}
	if len(report.TechStack) != 0 {
		t.Errorf("TechStack = %v, want empty", report.TechStack)
	}
	if report.HasCI || report.HasTests || report.HasDocker || report.HasIaC {
		t.Errorf("expected all structural flags false, got %+v", report)
	}
}

// A monorepo with no manifest at the tracked root (only nested
// under apps/frontend and apps/backend) must still detect both stacks.
func TestDetectStackMonorepoDetectsNestedManifests(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "apps/frontend/package.json", `{
		"dependencies": { "next": "^14.0.0" }
	}`)
	writeFile(t, dir, "apps/frontend/src/App.tsx", "export default function App() { return null }")
	writeFile(t, dir, "apps/backend/go.mod", "module example.com/svc\n\nrequire github.com/gin-gonic/gin v1.9.0\n")
	writeFile(t, dir, "apps/backend/main.go", "package main\nfunc main() {}\n")
	commitAll(t, dir)

	report, err := DetectStack(dir)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}

	if !contains(report.TechStack, "Next.js") {
		t.Errorf("TechStack = %v, want Next.js", report.TechStack)
	}
	if !contains(report.TechStack, "Gin") {
		t.Errorf("TechStack = %v, want Gin", report.TechStack)
	}
}

func TestDetectStackNotAGitRepoReturnsError(t *testing.T) {
	dir := t.TempDir() // no git init
	_, err := DetectStack(dir)
	if err == nil {
		t.Fatal("expected an error for a non-git directory")
	}
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
