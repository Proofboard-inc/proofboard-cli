package commands

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
)

func TestConfigBranchOperations(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	ctx := context.Background()

	// Initial watched branches should be the default ones
	// We run 'config branches'
	var out bytes.Buffer
	cmd := newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"branches"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config branches: %v", err)
	}

	expectedDefault := "main\nmaster\ndevelop\n"
	if out.String() != expectedDefault {
		t.Errorf("expected default branches %q, got %q", expectedDefault, out.String())
	}

	// Now add a branch
	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"add-branch", "feature-branch"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config add-branch: %v", err)
	}

	// Verify added
	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"branches"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config branches: %v", err)
	}
	expectedAfterAdd := "main\nmaster\ndevelop\nfeature-branch\n"
	if out.String() != expectedAfterAdd {
		t.Errorf("expected branches after add %q, got %q", expectedAfterAdd, out.String())
	}

	// Add duplicate branch (should not add duplicate)
	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"add-branch", "feature-branch"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config add-branch: %v", err)
	}

	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"branches"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config branches: %v", err)
	}
	if out.String() != expectedAfterAdd {
		t.Errorf("expected branches after duplicate add %q, got %q", expectedAfterAdd, out.String())
	}

	// Remove branch
	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"remove-branch", "master"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config remove-branch: %v", err)
	}

	// Verify removed
	out.Reset()
	cmd = newConfigCommand(ctx, &out)
	cmd.SetArgs([]string{"branches"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute config branches: %v", err)
	}
	expectedAfterRemove := "main\ndevelop\nfeature-branch\n"
	if out.String() != expectedAfterRemove {
		t.Errorf("expected branches after remove %q, got %q", expectedAfterRemove, out.String())
	}
}

func createTempGitRepo(t *testing.T) string {
	repoDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "init")
		cmd.Dir = repoDir
		_ = cmd.Run()
	}
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", repoDir, "remote", "add", "origin", "git@github.com:org/repo.git").Run()
	return repoDir
}

func TestSyncSuppressedWorkspace(t *testing.T) {
	tempHome := t.TempDir()
	repoDir := createTempGitRepo(t)

	t.Setenv("HOME", tempHome)

	ctx := context.Background()

	credStore := pbauth.NewCredentialStore(tempHome)
	err := credStore.Save(ctx, model.Credentials{
		Token:     "test-token",
		EmailHash: "test-email-hash",
	})
	if err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	stateStore := state.NewStore(tempHome)
	st, err := stateStore.Load(ctx)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	st, err = state.AddWorkspaceSuppression(st, repoDir)
	if err != nil {
		t.Fatalf("failed to suppress workspace: %v", err)
	}
	if err := stateStore.Save(ctx, st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	var out bytes.Buffer
	cmd := newSyncCommand(ctx, &out)
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("expected silent exit, got error: %v", err)
	}

	if out.Len() > 0 {
		t.Errorf("expected no output for suppressed workspace, got: %q", out.String())
	}
}
