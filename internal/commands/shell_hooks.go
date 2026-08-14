package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/logging"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

const (
	legacyShellDetectionLine = "proofboard detect >/dev/null 2>&1 &"

	// legacyBackgroundedShellDetectionLine is a prior value of
	// shellDetectionLine: it ran `detect` backgrounded with both stdout and
	// stderr sent to /dev/null, discarding the "New repository detected"
	// prompt it prints on a match (see printLinkDetected in detect.go) and
	// silently burning the one-time prompt for every workspace, since detect
	// records a prompt as permanently "shown" the moment it runs (see
	// printLinkDetected, which only records anything on an explicit "never").
	// `detect` only ever does fast, local git plumbing (no network) with its
	// own bounded timeout, so there is no real latency reason to background
	// or silence it; see shellDetectionLine below, which matches noticeLine's
	// pattern (synchronous, stderr-only suppression) instead.
	legacyBackgroundedShellDetectionLine = "(proofboard detect >/dev/null 2>&1 &)"
	legacyFishBackgroundedDetectionLine  = "proofboard detect >/dev/null 2>&1 &\ndisown $last_pid"

	// legacyPlainShellDetectionLine / legacyPlainNoticeLine are prior values
	// of shellDetectionLine/noticeLine. Every shell target installs its lines
	// into TWO rc files (e.g. zsh gets both ~/.zprofile and ~/.zshrc) so
	// detection still works regardless of which one a given terminal app
	// actually sources, but a new terminal window commonly sources BOTH (a
	// login shell reads ~/.zprofile, then an interactive shell reads
	// ~/.zshrc, in the same session), so an unguarded line in both files
	// would run `detect`/`notices` twice and print everything twice. The
	// guarded lines below wrap each command in a check against an exported
	// per-session env var, so the second file's copy is a no-op once the
	// first has already run in that shell.
	legacyPlainShellDetectionLine = "proofboard detect 2>/dev/null"
	legacyPlainNoticeLine         = "proofboard notices 2>/dev/null"
	legacyPlainPSDetectionLine    = "proofboard detect 2>$null"
	legacyPlainPSNoticeLine       = "proofboard notices 2>$null"

	shellDetectionLine     = `if [ -z "$PROOFBOARD_DETECTED" ]; then export PROOFBOARD_DETECTED=1; proofboard detect 2>/dev/null; fi`
	fishShellDetectionLine = "if not set -q PROOFBOARD_DETECTED; set -gx PROOFBOARD_DETECTED 1; proofboard detect 2>/dev/null; end"

	// noticeLine runs synchronously, same as shellDetectionLine above, so its
	// output is actually visible the moment a terminal starts up: the same
	// subtle "one line on shell startup" pattern as a venv auto-activation
	// hook. Only stderr is suppressed; a slow network is already bounded by a
	// short timeout inside the command itself.
	noticeLine     = `if [ -z "$PROOFBOARD_NOTICES_SHOWN" ]; then export PROOFBOARD_NOTICES_SHOWN=1; proofboard notices 2>/dev/null; fi`
	fishNoticeLine = "if not set -q PROOFBOARD_NOTICES_SHOWN; set -gx PROOFBOARD_NOTICES_SHOWN 1; proofboard notices 2>/dev/null; end"
	psNoticeLine   = `if (-not $env:PROOFBOARD_NOTICES_SHOWN) { $env:PROOFBOARD_NOTICES_SHOWN = "1"; proofboard notices 2>$null }`

	legacyPSDetectionLine = "Start-Process -WindowStyle Hidden -FilePath proofboard -ArgumentList 'detect' | Out-Null"
	psDetectionLine       = `if (-not $env:PROOFBOARD_DETECTED) { $env:PROOFBOARD_DETECTED = "1"; proofboard detect 2>$null }`
)

func newShellHookMaintenanceCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook-maintain",
		Hidden: true,
		Short:  "Maintain shell startup hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return maintainShellHooks(ctx)
		},
	}
	cmd.SetOut(out)
	return cmd
}

func maintainShellHooks(ctx context.Context) error {
	runCtx, err := loadRuntime(ctx)
	if err != nil {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	updated, inspected, err := ensureShellDetectionHooks(checkCtx)
	if err != nil {
		_ = logging.WriteSyncLog(runCtx.homeDir, "shell-hooks", "maintenance", "shell hook check", "failure", err.Error())
		return nil
	}
	if inspected == 0 {
		_ = logging.WriteSyncLog(runCtx.homeDir, "shell-hooks", "maintenance", "shell hook check", "skipped", "no matching profile found")
		return nil
	}
	if updated {
		_ = logging.WriteSyncLog(runCtx.homeDir, "shell-hooks", "maintenance", "shell hook check", "success", "workspace detection hook installed")
		return nil
	}
	_ = logging.WriteSyncLog(runCtx.homeDir, "shell-hooks", "maintenance", "shell hook check", "skipped", "workspace detection hook already present")
	return nil
}

func ensureShellDetectionHooks(ctx context.Context) (updated bool, inspected int, err error) {
	targets, err := shellHookTargets()
	if err != nil {
		return false, 0, err
	}
	for _, target := range targets {
		inspected++
		if target.Path == "" {
			continue
		}
		for _, line := range target.Lines {
			done, _, err := ensureLineInFile(target.Path, line)
			if err != nil {
				return false, inspected, err
			}
			if done {
				updated = true
			}
		}
	}
	// Attempt recovery on every call, not just when this exact call catches
	// a legacy line mid-migration: an install may already have had its rc
	// file migrated to the synchronous line by an earlier CLI run, leaving
	// burned PromptedWorkspaces entries with no other reliable signal left to
	// catch. It is idempotent and near-free once RecoveredLegacyPrompts is
	// set (see below), so calling it unconditionally here is cheap. Best
	// effort: a failure must never block hook maintenance itself.
	_ = recoverBurnedWorkspacePrompts(ctx)
	return updated, inspected, nil
}

// recoverBurnedWorkspacePrompts undoes the "detect silently burns the
// one-time prompt" bug caused by any prior CLI version's backgrounded/
// silenced hook line. See statestore.RecoverBurnedWorkspacePrompts for the
// full rationale.
func recoverBurnedWorkspacePrompts(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	store := statestore.NewStore(homeDir)
	current, err := store.Load(ctx)
	if err != nil {
		return err
	}
	recovered := statestore.RecoverBurnedWorkspacePrompts(current)
	if recovered.RecoveredLegacyPrompts == current.RecoveredLegacyPrompts {
		return nil
	}
	return store.Save(ctx, recovered)
}

// legacyDetectionLines are every prior value of the shell startup detection
// line, oldest first, checked in ensureLineInFile so an existing install
// gets migrated onto the current line instead of ending up with two, or
// silently keeping a broken backgrounded/silenced one forever because it
// never matches "line already present".
var legacyDetectionLines = []string{
	legacyShellDetectionLine,
	legacyBackgroundedShellDetectionLine,
	legacyFishBackgroundedDetectionLine,
	legacyPSDetectionLine,
	legacyPlainShellDetectionLine,
	legacyPlainNoticeLine,
	legacyPlainPSDetectionLine,
	legacyPlainPSNoticeLine,
}

// containsWholeLine reports whether legacy appears in content bounded by
// newlines (or start/end of content) on both sides, i.e. as its own
// previously-written line, not merely as a text fragment. A plain substring
// check is not safe here: legacyPlainShellDetectionLine/legacyPlainNoticeLine
// ("proofboard detect 2>/dev/null" / "proofboard notices 2>/dev/null") are
// themselves substrings of the current guarded shellDetectionLine/noticeLine
// values (the guard wraps the exact same command). Without this boundary
// check, ensureLineInFile would treat an already-correct, already-written
// guarded line as a "legacy" match on every subsequent call and splice a
// second copy of the other line into the middle of it, growing without bound
// each time hook maintenance runs.
func containsWholeLine(content, legacy string) bool {
	idx := strings.Index(content, legacy)
	if idx == -1 {
		return false
	}
	if idx > 0 && content[idx-1] != '\n' {
		return false
	}
	end := idx + len(legacy)
	if end < len(content) && content[end] != '\n' {
		return false
	}
	return true
}

// ensureLineInFile makes sure `line` is present in the rc file at `path`,
// migrating it in place from any known legacy value first. migratedLegacy
// reports specifically whether an old backgrounded/silenced hook line was
// found and replaced. The caller uses this to trigger burned-prompt
// recovery (see recoverBurnedWorkspacePrompts), since every legacy line in
// legacyDetectionLines silenced `detect`'s output while still recording its
// one-time prompt as shown.
func ensureLineInFile(path string, line string) (updated bool, migratedLegacy bool, err error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, false, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(content), line) {
		return false, false, nil
	}
	for _, legacy := range legacyDetectionLines {
		if !containsWholeLine(string(content), legacy) {
			continue
		}
		migrated := strings.ReplaceAll(string(content), legacy, line)
		mode := os.FileMode(0o644)
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path, []byte(migrated), mode); err != nil {
			return false, false, fmt.Errorf("migrate %s: %w", path, err)
		}
		return true, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, false, fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n# Proofboard Workspace Detection\n%s\n", line); err != nil {
		return false, false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, false, nil
}

type detectHookTarget struct {
	Path  string
	Lines []string
}

func shellHookTargets() ([]detectHookTarget, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory: %w", err)
	}

	shell := strings.ToLower(filepath.Base(os.Getenv("SHELL")))
	if shell == "" && os.Getenv("OS") == "Windows_NT" {
		shell = "powershell"
	}

	switch shell {
	case "bash":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".bashrc"), Lines: []string{shellDetectionLine, noticeLine}},
			{Path: filepath.Join(homeDir, ".bash_profile"), Lines: []string{shellDetectionLine, noticeLine}},
		}, nil
	case "zsh":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".zshrc"), Lines: []string{shellDetectionLine, noticeLine}},
			{Path: filepath.Join(homeDir, ".zprofile"), Lines: []string{shellDetectionLine, noticeLine}},
		}, nil
	case "fish":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".config", "fish", "config.fish"), Lines: []string{fishShellDetectionLine, fishNoticeLine}},
		}, nil
	case "powershell", "pwsh":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine, psNoticeLine}},
			{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine, psNoticeLine}},
		}, nil
	default:
		if os.Getenv("OS") == "Windows_NT" {
			return []detectHookTarget{
				{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine, psNoticeLine}},
				{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine, psNoticeLine}},
			}, nil
		}
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".profile"), Lines: []string{shellDetectionLine, noticeLine}},
		}, nil
	}
}
