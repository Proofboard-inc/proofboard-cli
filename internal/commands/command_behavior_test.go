package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
)

func TestLinkAndUnlinkCommandLifecycle(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	writeRepoFileAndCommit(t, repoDir)
	head := gitOutput(t, repoDir, "rev-parse", "HEAD")
	runGit(t, repoDir, "update-ref", "refs/remotes/origin/main", head)
	runGit(t, repoDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/link" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode link request: %v", err)
		}
		encoded, _ := json.Marshal(payload)
		for _, proprietary := range []string{"org/repo", "github.com", repoDir} {
			if strings.Contains(string(encoded), proprietary) {
				t.Fatalf("link request leaked %q: %s", proprietary, encoded)
			}
		}
		if payload["orgHash"] == "" || payload["repoHash"] == "" || payload["provider"] != "github" {
			t.Fatalf("link request missing anonymized identity: %s", encoded)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isNewProject":      false,
			"projectId":         "project-123",
			"dictionaryVersion": "1.0.0",
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_API_LINK_PATH", "/link")

	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{Token: "test-token", EmailHash: "email-hash"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	restoreWorkingDirectory(t, repoDir)

	var linkOut bytes.Buffer
	linkCommand := newLinkCommand(ctx, &linkOut)
	linkCommand.SetArgs([]string{"--non-interactive"})
	if err := linkCommand.ExecuteContext(ctx); err != nil {
		t.Fatalf("link command: %v\n%s", err, linkOut.String())
	}
	current, err := statestore.NewStore(homeDir).Load(ctx)
	if err != nil {
		t.Fatalf("load linked state: %v", err)
	}
	if len(current.LinkedRepos) != 1 {
		t.Fatalf("linked repositories = %d, want 1", len(current.LinkedRepos))
	}
	for _, hook := range []string{"post-commit", "post-merge", "post-rewrite"} {
		if _, err := os.Stat(filepath.Join(repoDir, ".git", "hooks", hook)); err != nil {
			t.Fatalf("%s hook not installed: %v", hook, err)
		}
	}

	var unlinkOut bytes.Buffer
	if err := newUnlinkCommand(ctx, &unlinkOut).ExecuteContext(ctx); err != nil {
		t.Fatalf("unlink command: %v", err)
	}
	current, err = statestore.NewStore(homeDir).Load(ctx)
	if err != nil {
		t.Fatalf("load unlinked state: %v", err)
	}
	if len(current.LinkedRepos) != 0 {
		t.Fatalf("linked repositories after unlink = %d", len(current.LinkedRepos))
	}
	for _, hook := range []string{"post-commit", "post-merge", "post-rewrite"} {
		if _, err := os.Stat(filepath.Join(repoDir, ".git", "hooks", hook)); !os.IsNotExist(err) {
			t.Fatalf("%s hook remains after unlink", hook)
		}
	}
}

func TestLogsCommandPrintsRequestedTail(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	path := filepath.Join(homeDir, ".proofboard", "sync.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create log directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var out bytes.Buffer
	command := newLogsCommand(context.Background(), &out)
	command.SetArgs([]string{"--lines", "2"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("logs command: %v", err)
	}
	if out.String() != "second\nthird\n" {
		t.Fatalf("logs output = %q", out.String())
	}
}

func TestCompletionAndVersionCommands(t *testing.T) {
	t.Setenv("PROOFBOARD_DISABLE_STARTUP_CHECKS", "1")
	t.Setenv("PROOFBOARD_DISABLE_SHELL_HOOK_MAINTENANCE", "1")
	var completionOut bytes.Buffer
	root := NewRootCommand(context.Background(), &completionOut, &completionOut)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("completion command: %v", err)
	}
	if !strings.Contains(completionOut.String(), "__start_proofboard") {
		t.Fatal("bash completion output does not contain the root completion function")
	}

	var versionOut bytes.Buffer
	if err := newVersionCommand(context.Background(), &versionOut).ExecuteContext(context.Background()); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if !strings.HasPrefix(versionOut.String(), "proofboard version 1.") {
		t.Fatalf("version output = %q", versionOut.String())
	}
}

func TestInstallAndUninstallCommandsUseInjectedActions(t *testing.T) {
	var installOut bytes.Buffer
	installCalls := 0
	installCommand := newInstallCommandWithAction(func(out io.Writer) error {
		installCalls++
		_, err := io.WriteString(out, "installed safely\n")
		return err
	})
	installCommand.SetOut(&installOut)
	if err := installCommand.Execute(); err != nil {
		t.Fatalf("install command: %v", err)
	}
	if installCalls != 1 || installOut.String() != "installed safely\n" {
		t.Fatalf("install calls/output = %d, %q", installCalls, installOut.String())
	}

	var uninstallOut bytes.Buffer
	uninstallCalls := 0
	uninstallCommand := newUninstallCommandWithAction(func(out io.Writer) error {
		uninstallCalls++
		_, err := io.WriteString(out, "uninstalled safely\n")
		return err
	})
	uninstallCommand.SetOut(&uninstallOut)
	if err := uninstallCommand.Execute(); err != nil {
		t.Fatalf("uninstall command: %v", err)
	}
	if uninstallCalls != 1 || uninstallOut.String() != "uninstalled safely\n" {
		t.Fatalf("uninstall calls/output = %d, %q", uninstallCalls, uninstallOut.String())
	}
}

func TestAgentStatusCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	command := newAgentCommand(context.Background(), &out)
	command.SetArgs([]string{"status"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("agent status: %v", err)
	}
	if !strings.Contains(out.String(), "Proofboard Career Agent: Stopped") {
		t.Fatalf("agent status output = %q", out.String())
	}
}

func writeRepoFileAndCommit(t *testing.T, repoDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# local test\n"), 0o600); err != nil {
		t.Fatalf("write repository file: %v", err)
	}
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "local test commit")
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repoDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func gitOutput(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repoDir
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

func restoreWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
}
