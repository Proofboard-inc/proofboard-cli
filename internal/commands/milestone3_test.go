package commands

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

func TestLastFridayCalculation(t *testing.T) {
	tests := []struct {
		now               time.Time
		expectedKey       string
		expectedMonthName string
	}{
		{
			// June 16, 2026. Last Friday of June 2026 is June 26.
			// 16 is before 26, so previous month (May) is ready.
			now:               time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC),
			expectedKey:       "2026-05",
			expectedMonthName: "May",
		},
		{
			// June 27, 2026. Last Friday of June 2026 is June 26.
			// 27 is after 26, so current month (June) is ready.
			now:               time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC),
			expectedKey:       "2026-06",
			expectedMonthName: "June",
		},
		{
			// June 26, 2026 at 00:00:01 (after 00:00:00 start of Friday).
			// Last Friday is ready.
			now:               time.Date(2026, time.June, 26, 0, 0, 1, 0, time.UTC),
			expectedKey:       "2026-06",
			expectedMonthName: "June",
		},
		{
			// February 2026. Last Friday of Feb 2026 is Feb 27.
			// Feb 28 is after 27, current month (Feb) is ready.
			now:               time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC),
			expectedKey:       "2026-02",
			expectedMonthName: "February",
		},
		{
			// February 2026. Feb 26 is before 27, previous month (Jan) is ready.
			now:               time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC),
			expectedKey:       "2026-01",
			expectedMonthName: "January",
		},
	}

	for _, tc := range tests {
		t.Run(tc.now.Format("2006-01-02"), func(t *testing.T) {
			key, monthName := getReadyCareerSummaryMonth(tc.now)
			if key != tc.expectedKey {
				t.Errorf("getReadyCareerSummaryMonth(%v) key = %q, want %q", tc.now, key, tc.expectedKey)
			}
			if monthName != tc.expectedMonthName {
				t.Errorf("getReadyCareerSummaryMonth(%v) monthName = %q, want %q", tc.now, monthName, tc.expectedMonthName)
			}
		})
	}
}

func TestCareerSummaryNotificationTrigger(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	ctx := context.Background()
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("loadRuntime: %v", err)
	}

	// Trigger on June 16, 2026 (May is ready)
	var out bytes.Buffer
	now1 := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)
	if err := triggerMonthlyCareerSummaryWithTime(ctx, &out, runtime, now1); err != nil {
		t.Fatalf("triggerMonthlyCareerSummaryWithTime: %v", err)
	}

	expectedMsg1 := "Proofboard: Your May career summary is ready. proofboard.io/career-summary\n"
	if out.String() != expectedMsg1 {
		t.Errorf("expected notification %q, got %q", expectedMsg1, out.String())
	}

	// Trigger again on same day (should be quiet)
	out.Reset()
	if err := triggerMonthlyCareerSummaryWithTime(ctx, &out, runtime, now1); err != nil {
		t.Fatalf("triggerMonthlyCareerSummaryWithTime: %v", err)
	}
	if out.Len() > 0 {
		t.Errorf("expected no notification on repeat, got %q", out.String())
	}

	// Trigger on June 27, 2026 (June is ready)
	out.Reset()
	now2 := time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC)
	if err := triggerMonthlyCareerSummaryWithTime(ctx, &out, runtime, now2); err != nil {
		t.Fatalf("triggerMonthlyCareerSummaryWithTime: %v", err)
	}

	expectedMsg2 := "Proofboard: Your June career summary is ready. proofboard.io/career-summary\n"
	if out.String() != expectedMsg2 {
		t.Errorf("expected notification %q, got %q", expectedMsg2, out.String())
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
	repoPathAbs, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	st.SuppressedWorkspaces = append(st.SuppressedWorkspaces, repoPathAbs)
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
