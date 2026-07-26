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
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/logging"
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
			"emailHashKey":      testEmailHashKey,
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
	for _, linked := range current.LinkedRepos {
		if linked.EmailHashKey != testEmailHashKey {
			t.Fatalf("linked repository did not retain emailHashKey")
		}
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

func TestLinkMigratesLegacyStateWithHandshakeBeforeProjectSelection(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := createTempGitRepo(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	const legacyProjectID = "legacy-project-123"
	repoHash := crypto.SHA256("github:org/repo")
	orgHash := crypto.SHA256("github:org")
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cli/repos/link" {
			http.NotFound(w, r)
			return
		}
		requestCount++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode link request: %v", err)
		}
		if requestCount == 1 {
			if request["existingProjectId"] != nil {
				http.Error(w, `{"message":"legacy project mapping not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isNewProject": true,
				"existingProjectOptions": []map[string]string{{
					"id": legacyProjectID,
				}},
			})
			return
		}
		if request["existingProjectId"] != legacyProjectID || request["createNew"] == true {
			t.Fatalf("legacy selection request = %#v", request)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isNewProject":      false,
			"projectId":         legacyProjectID,
			"dictionaryVersion": "1.2.0",
			"emailHashKey":      testEmailHashKey,
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)

	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{Token: "test-token"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	current := statestore.Default()
	current.LinkedRepos[repoHash] = model.LinkedRepoState{
		RepoHash:  repoHash,
		OrgHash:   orgHash,
		Provider:  "github",
		ProjectID: legacyProjectID,
	}
	if err := statestore.NewStore(homeDir).Save(ctx, current); err != nil {
		t.Fatalf("save legacy state: %v", err)
	}
	restoreWorkingDirectory(t, repoDir)

	var out bytes.Buffer
	command := newLinkCommand(ctx, &out)
	command.SetArgs([]string{"--non-interactive"})
	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatalf("migrate legacy link: %v\n%s", err, out.String())
	}
	if requestCount != 2 {
		t.Fatalf("link request count = %d, want handshake plus selection", requestCount)
	}
	persisted, err := statestore.NewStore(homeDir).Load(ctx)
	if err != nil {
		t.Fatalf("load migrated state: %v", err)
	}
	linked := persisted.LinkedRepos[repoHash]
	if linked.ProjectID != legacyProjectID || linked.EmailHashKey != testEmailHashKey {
		t.Fatalf("migrated state = %+v", linked)
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

func TestLogsClearPurgesCurrentAndRotatedLogs(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	path := filepath.Join(homeDir, ".proofboard", "sync.log")
	backupPath := path + ".1"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create log directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("current private log\n"), 0o600); err != nil {
		t.Fatalf("write current log: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("rotated private log\n"), 0o600); err != nil {
		t.Fatalf("write rotated log: %v", err)
	}

	var out bytes.Buffer
	command := newLogsCommand(context.Background(), &out)
	command.SetArgs([]string{"clear"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("logs clear command: %v", err)
	}
	if out.String() != "Proofboard logs cleared.\n" {
		t.Fatalf("logs clear output = %q", out.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat reset log: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("reset log size = %d, want 0", info.Size())
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("reset log mode = %o, want 600", info.Mode().Perm())
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("rotated log still exists or stat failed: %v", err)
	}
	if err := logging.WriteSyncLog(homeDir, "repo-hash", "test", "after clear", "success", ""); err != nil {
		t.Fatalf("write fresh log entry: %v", err)
	}
	fresh, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fresh log: %v", err)
	}
	if !bytes.Contains(fresh, []byte("after clear")) ||
		bytes.Contains(fresh, []byte("current private log")) ||
		bytes.Contains(fresh, []byte("rotated private log")) {
		t.Fatalf("fresh log contents = %q", fresh)
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

func TestEveryAgentLifecycleCommandUsesItsAction(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bare status", want: "status"},
		{name: "run", args: []string{"run"}, want: "run"},
		{name: "enable", args: []string{"enable"}, want: "enable"},
		{name: "disable", args: []string{"disable"}, want: "disable"},
		{name: "start", args: []string{"start"}, want: "start"},
		{name: "stop", args: []string{"stop"}, want: "stop"},
		{name: "status", args: []string{"status"}, want: "status"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			record := func(name string) {
				got = append(got, name)
			}
			actions := agentCommandActions{
				run: func(context.Context) error {
					record("run")
					return nil
				},
				enable: func(io.Writer) error {
					record("enable")
					return nil
				},
				disable: func(io.Writer) error {
					record("disable")
					return nil
				},
				start: func(context.Context, io.Writer) error {
					record("start")
					return nil
				},
				stop: func(io.Writer) error {
					record("stop")
					return nil
				},
				status: func(context.Context, io.Writer) error {
					record("status")
					return nil
				},
			}
			command := newAgentCommandWithActions(ctx, io.Discard, actions)
			command.SetArgs(test.args)
			if err := command.ExecuteContext(ctx); err != nil {
				t.Fatalf("agent %v: %v", test.args, err)
			}
			if len(got) != 1 || got[0] != test.want {
				t.Fatalf("agent %v actions = %v, want [%s]", test.args, got, test.want)
			}
		})
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
