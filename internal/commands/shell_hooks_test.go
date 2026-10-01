package commands

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	statestore "github.com/proofboard/proofboard/internal/state"
)

func TestEnsureLineInFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")

	updated, migratedLegacy, err := ensureLineInFile(path, shellDetectionLine(defaultHookCommand))
	if err != nil {
		t.Fatalf("ensureLineInFile first write failed: %v", err)
	}
	if !updated {
		t.Fatalf("expected first write to report updated")
	}
	if migratedLegacy {
		t.Fatalf("expected first write (no prior file) to not report a legacy migration")
	}

	updated, migratedLegacy, err = ensureLineInFile(path, shellDetectionLine(defaultHookCommand))
	if err != nil {
		t.Fatalf("ensureLineInFile second write failed: %v", err)
	}
	if updated {
		t.Fatalf("expected second write to be a no-op")
	}
	if migratedLegacy {
		t.Fatalf("expected no-op second write to not report a legacy migration")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}

	content := string(data)
	if strings.Count(content, shellDetectionLine(defaultHookCommand)) != 1 {
		t.Fatalf("expected exactly one detection hook, got content: %q", content)
	}
}

func TestEnsureLineInFile_MigratesTrackedBackgroundJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	content := "# Proofboard Workspace Detection\n" + legacyShellDetectionLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("write legacy hook: %v", err)
	}

	updated, migratedLegacy, err := ensureLineInFile(path, shellDetectionLine(defaultHookCommand))
	if err != nil || !updated {
		t.Fatalf("migration = %v, %v", updated, err)
	}
	if !migratedLegacy {
		t.Fatalf("expected migration from a legacy hook line to be reported")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated hook: %v", err)
	}
	if strings.Contains(string(data), "\n"+legacyShellDetectionLine+"\n") {
		t.Fatalf("legacy hook remains: %q", data)
	}
	if strings.Count(string(data), shellDetectionLine(defaultHookCommand)) != 1 {
		t.Fatalf("migrated hook count = %d", strings.Count(string(data), shellDetectionLine(defaultHookCommand)))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat migrated hook: %v", err)
	}
	// Windows has no Unix permission bits; os.Stat reports 0666 there whatever
	// the ACL says, so this compares nothing and fails for an unrelated reason.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

// Regression test for the "detect never actually prints anything" bug: an
// install whose rc file still has the old backgrounded-and-fully-silenced
// line (`(proofboard detect >/dev/null 2>&1 &)`, both stdout AND stderr
// discarded) must get migrated onto the new synchronous line, not left with
// a hook that can never surface a prompt.
func TestEnsureLineInFile_MigratesLegacyBackgroundedDetectLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := "# Proofboard Workspace Detection\n" + legacyBackgroundedShellDetectionLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("write legacy hook: %v", err)
	}

	updated, migratedLegacy, err := ensureLineInFile(path, shellDetectionLine(defaultHookCommand))
	if err != nil || !updated {
		t.Fatalf("migration = %v, %v", updated, err)
	}
	if !migratedLegacy {
		t.Fatalf("expected migration from a legacy backgrounded hook line to be reported")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated hook: %v", err)
	}
	if strings.Contains(string(data), legacyBackgroundedShellDetectionLine) {
		t.Fatalf("legacy backgrounded hook remains: %q", data)
	}
	if !strings.Contains(string(data), shellDetectionLine(defaultHookCommand)) {
		t.Fatalf("expected migrated hook to contain %q, got: %q", shellDetectionLine(defaultHookCommand), data)
	}
	// The old line discarded stdout too (">/dev/null 2>&1", plus backgrounded
	// with "&"), the new one must only suppress stderr and run synchronously.
	if strings.Contains(shellDetectionLine(defaultHookCommand), "1>/dev/null") || strings.Contains(shellDetectionLine(defaultHookCommand), "2>&1") || strings.HasSuffix(strings.TrimSpace(shellDetectionLine(defaultHookCommand)), "&") {
		t.Fatalf("new detect line must not discard stdout or run backgrounded: %q", shellDetectionLine(defaultHookCommand))
	}
}

func TestEnsureShellDetectionHooksWritesBothDetectAndNoticeLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")

	for i := 0; i < 2; i++ {
		for _, line := range []string{shellDetectionLine(defaultHookCommand), noticeLine(defaultHookCommand)} {
			if _, _, err := ensureLineInFile(path, line); err != nil {
				t.Fatalf("ensureLineInFile(%q) run %d: %v", line, i, err)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	content := string(data)
	if strings.Count(content, shellDetectionLine(defaultHookCommand)) != 1 {
		t.Fatalf("expected exactly one detect hook, got: %q", content)
	}
	if strings.Count(content, noticeLine(defaultHookCommand)) != 1 {
		t.Fatalf("expected exactly one notice hook, got: %q", content)
	}
	// The notice line must run synchronously (not backgrounded/silenced the
	// way detect is) so its output is actually visible on shell startup.
	if strings.Contains(noticeLine(defaultHookCommand), ">/dev/null 2>&1 &") {
		t.Fatalf("notice line must not be backgrounded/fully silenced: %q", noticeLine(defaultHookCommand))
	}
}

// Regression test for the "upgrading the hook doesn't bring back the
// message" bug: a developer whose rc file still had the legacy backgrounded/
// silenced detect line had every not-yet-linked workspace silently marked
// "prompted" without ever seeing the message. Running the (now-fixed) hook
// maintenance must also clear those burned prompt markers, once, so the
// fixed synchronous hook can actually surface "New repository detected"
// again.
func TestEnsureShellDetectionHooks_RecoversBurnedPromptsOnLegacyMigration(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("SHELL", "/bin/bash")

	rcPath := filepath.Join(homeDir, ".bashrc")
	if err := os.WriteFile(rcPath, []byte("# Proofboard Workspace Detection\n"+legacyBackgroundedShellDetectionLine+"\n"), 0o644); err != nil {
		t.Fatalf("seed legacy rc file: %v", err)
	}

	store := statestore.NewStore(homeDir)
	seeded := statestore.Default()
	seeded.PromptedWorkspaces = map[string]time.Time{
		"burned-workspace-key": time.Now().UTC(),
	}
	if err := store.Save(context.Background(), seeded); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
		t.Fatalf("ensureShellDetectionHooks: %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load recovered state: %v", err)
	}
	if len(got.PromptedWorkspaces) != 0 {
		t.Fatalf("expected burned prompt markers to be cleared, got: %v", got.PromptedWorkspaces)
	}
	if !got.RecoveredLegacyPrompts {
		t.Fatalf("expected RecoveredLegacyPrompts to be set after recovery")
	}
}

// Most real-world affected installs already had their rc file silently
// migrated to the new synchronous line by an earlier CLI run, well before
// this fix existed, so by the time a developer upgrades, there is no legacy
// line left to catch. Recovery must still run and heal already-burned state
// in that case, not only when it happens to observe a live migration.
func TestEnsureShellDetectionHooks_RecoversBurnedPromptsEvenWithoutLiveLegacyLine(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("SHELL", "/bin/bash")

	rcPath := filepath.Join(homeDir, ".bashrc")
	if err := os.WriteFile(rcPath, []byte("# Proofboard Workspace Detection\n"+shellDetectionLine(defaultHookCommand)+"\n"), 0o644); err != nil {
		t.Fatalf("seed already-migrated rc file: %v", err)
	}

	store := statestore.NewStore(homeDir)
	seeded := statestore.Default()
	seeded.PromptedWorkspaces = map[string]time.Time{
		"burned-workspace-key": time.Now().UTC(),
	}
	if err := store.Save(context.Background(), seeded); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
		t.Fatalf("ensureShellDetectionHooks: %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load recovered state: %v", err)
	}
	if len(got.PromptedWorkspaces) != 0 {
		t.Fatalf("expected burned prompt markers to be cleared even without a live legacy migration, got: %v", got.PromptedWorkspaces)
	}
	if !got.RecoveredLegacyPrompts {
		t.Fatalf("expected RecoveredLegacyPrompts to be set after recovery")
	}
}

// FIX: hook lines used to call `proofboard` by bare name, resolved via
// $PATH at the moment the shell ran the line. GUI apps (VS Code, Zed, ...)
// resolve and cache their own process environment once, at their own
// launch, and never revisit it for a new window/terminal opened inside
// that same already-running process — so a shell open before an
// install/reinstall wrote a fresh $PATH would never see `proofboard` at
// all, silently (every hook line redirects stderr to /dev/null). Hooks
// must instead call the CLI by its own resolved absolute path, which needs
// no $PATH lookup at all.
func TestResolveHookBinaryPathReturnsAnAbsolutePath(t *testing.T) {
	got := resolveHookBinaryPath()
	if got == defaultHookCommand {
		t.Fatalf("resolveHookBinaryPath fell back to the bare command name in a normal test environment: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveHookBinaryPath did not return an absolute path: %q", got)
	}
}

func TestEnsureShellDetectionHooksWriteAbsoluteBinaryPathNotBareCommand(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("SHELL", "/bin/zsh")

	if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
		t.Fatalf("ensureShellDetectionHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	content := string(data)

	wantBin := resolveHookBinaryPath()
	if wantBin == defaultHookCommand {
		t.Fatalf("test environment could not resolve an absolute binary path")
	}
	if !strings.Contains(content, wantBin) {
		t.Fatalf(".zshrc does not invoke the CLI by its absolute path %q: %q", wantBin, content)
	}
	// The old bare, unqualified invocation must be gone, not merely
	// supplemented by the new one — a duplicate still relying on $PATH
	// would still fail silently in a stale-environment shell.
	if strings.Contains(content, `"proofboard" detect`) || strings.Contains(content, " proofboard detect") {
		t.Fatalf(".zshrc still contains a bare, $PATH-dependent invocation: %q", content)
	}
}

// FIX: installTo used to call the self-healing ensureShellDetectionHooks,
// which resolves the hook binary via os.Executable() — the *currently
// running* process. During install that is whatever copied the executable
// into place (a temp download location from the install script, or a dev
// build run manually from a repo checkout), not the just-written
// destination. Hooks ended up pointing at a path that may not even exist
// once the installer's temp files were cleaned up — the exact bug the
// absolute-path fix was supposed to close. ensureShellDetectionHooksForBinary
// must use the explicitly supplied path, not silently fall back to
// os.Executable().
func TestEnsureShellDetectionHooksForBinaryUsesGivenPathNotRunningProcess(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("SHELL", "/bin/zsh")

	const explicitBin = "/some/explicit/final/install/location/proofboard"
	if _, _, err := ensureShellDetectionHooksForBinary(context.Background(), explicitBin); err != nil {
		t.Fatalf("ensureShellDetectionHooksForBinary: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, explicitBin) {
		t.Fatalf(".zshrc does not reference the explicitly supplied binary path %q: %q", explicitBin, content)
	}
	if runningBin := resolveHookBinaryPath(); runningBin != defaultHookCommand && strings.Contains(content, runningBin) {
		t.Fatalf("hooks reference the running test binary's own path (%q) instead of the explicitly supplied install location — the override was silently ignored: %q", runningBin, content)
	}
}

// FIX: a stale hook block — written by an earlier CLI version's template,
// or pointing at a binary that has since moved to a new install location —
// used to sit forever unmatched by ensureLineInFile's exact-content check,
// while a second, current block got appended below it: duplicated text
// that still worked (each copy is separately guarded) but never cleaned
// itself up. ensureShellDetectionHooks must replace a stale block in place
// instead of accumulating a second one alongside it.
func TestEnsureShellDetectionHooksReplacesStaleBlockRatherThanDuplicating(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("SHELL", "/bin/zsh")

	rcPath := filepath.Join(homeDir, ".zshrc")
	staleContent := strings.Join([]string{
		workspaceDetectionHeader,
		shellDetectionLine("/an/old/install/location/proofboard"),
		"",
		workspaceDetectionHeader,
		noticeLine("/an/old/install/location/proofboard"),
		"",
		workspaceDetectionHeader,
		zshChpwdHook("/an/old/install/location/proofboard"),
		"",
	}, "\n")
	if err := os.WriteFile(rcPath, []byte(staleContent), 0o644); err != nil {
		t.Fatalf("seed stale rc file: %v", err)
	}

	if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
		t.Fatalf("ensureShellDetectionHooks: %v", err)
	}

	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "/an/old/install/location/proofboard") {
		t.Fatalf("stale block pointing at the old binary location was not removed: %q", content)
	}
	if strings.Count(content, "PROOFBOARD_CHPWD_INSTALLED=1") != 1 {
		t.Fatalf("expected exactly one chpwd hook after healing a stale block, got: %q", content)
	}
}

func TestIsInternalCommand(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"notify":            {args: []string{"notify"}, want: true},
		"notify-activate":   {args: []string{"notify-activate"}, want: true},
		"notices":           {args: []string{"notices"}, want: true},
		"milestone-action":  {args: []string{"milestone-action"}, want: true},
		"hook-maintain":     {args: []string{"hook-maintain"}, want: true},
		"agent":             {args: []string{"agent"}, want: true},
		"update":            {args: []string{"update"}, want: true},
		"update-dictionary": {args: []string{"update-dictionary"}, want: true},
		"help":              {args: []string{"help"}, want: true},
		"sync":              {args: []string{"sync"}, want: false},
		"empty":             {args: nil, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isInternalCommand(tc.args); got != tc.want {
				t.Fatalf("isInternalCommand(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
