package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

	legacyPSDetectionLine = "Start-Process -WindowStyle Hidden -FilePath proofboard -ArgumentList 'detect' | Out-Null"

	// defaultHookCommand is the fallback invocation used when the running
	// binary's own absolute path cannot be resolved (see
	// resolveHookBinaryPath). It reproduces this CLI's long-standing
	// behavior of calling `proofboard` by bare name, relying on $PATH.
	defaultHookCommand = "proofboard"
)

// resolveHookBinaryPath returns the absolute path to the currently-running
// proofboard binary, for use in shell hook lines.
//
// Hook lines call the CLI by this absolute path rather than by bare name
// specifically so they do not depend on $PATH having been (re)resolved by
// the shell. GUI apps — VS Code, Zed, and similar — resolve and cache their
// own process environment once, at the app process's own launch, and never
// revisit it for a new window or a new integrated terminal opened inside
// that same still-running process. A hook line that depended on bare
// `proofboard` + $PATH would silently do nothing in any such
// already-running app (every hook line redirects stderr to /dev/null) until
// the whole app was quit and relaunched — which is exactly what "the
// detection prompt never shows up in VS Code" turned out to be. An absolute
// path sidesteps $PATH lookup entirely, so it works immediately regardless
// of when the app itself last resolved its environment.
//
// Falls back to defaultHookCommand (today's long-standing bare-name
// behavior) if the executable's path cannot be resolved for any reason —
// this must never fail hook maintenance itself.
func resolveHookBinaryPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return defaultHookCommand
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	abs, err := filepath.Abs(execPath)
	if err != nil {
		return defaultHookCommand
	}
	return abs
}

// posixInvoke quotes bin for use as a command in POSIX shells (bash, zsh,
// sh) and fish, so a path containing spaces still works. Quoting a bare
// command name (the defaultHookCommand fallback) is harmless — the shell
// still resolves it via $PATH exactly as if it were unquoted.
func posixInvoke(bin string) string {
	return `"` + bin + `"`
}

func shellDetectionLine(bin string) string {
	return fmt.Sprintf(`if [ -z "$PROOFBOARD_DETECTED" ]; then export PROOFBOARD_DETECTED=1; %s detect 2>/dev/null; fi`, posixInvoke(bin))
}

func fishShellDetectionLine(bin string) string {
	return fmt.Sprintf("if not set -q PROOFBOARD_DETECTED; set -gx PROOFBOARD_DETECTED 1; %s detect 2>/dev/null; end", posixInvoke(bin))
}

// noticeLine runs synchronously, same as shellDetectionLine above, so its
// output is actually visible the moment a terminal starts up: the same
// subtle "one line on shell startup" pattern as a venv auto-activation
// hook. Only stderr is suppressed; a slow network is already bounded by a
// short timeout inside the command itself.
func noticeLine(bin string) string {
	return fmt.Sprintf(`if [ -z "$PROOFBOARD_NOTICES_SHOWN" ]; then export PROOFBOARD_NOTICES_SHOWN=1; %s notices 2>/dev/null; fi`, posixInvoke(bin))
}

func fishNoticeLine(bin string) string {
	return fmt.Sprintf("if not set -q PROOFBOARD_NOTICES_SHOWN; set -gx PROOFBOARD_NOTICES_SHOWN 1; %s notices 2>/dev/null; end", posixInvoke(bin))
}

func psNoticeLine(bin string) string {
	return fmt.Sprintf(`if (-not $env:PROOFBOARD_NOTICES_SHOWN) { $env:PROOFBOARD_NOTICES_SHOWN = "1"; & "%s" notices 2>$null }`, bin)
}

// psDetectionLine, like every PowerShell hook line here, uses the `&` call
// operator with a quoted path — this invokes correctly whether bin is a
// bare command name (resolved via PATH, same as an unquoted call) or an
// absolute path, including one containing spaces.
func psDetectionLine(bin string) string {
	return fmt.Sprintf(`if (-not $env:PROOFBOARD_DETECTED) { $env:PROOFBOARD_DETECTED = "1"; & "%s" detect 2>$null }`, bin)
}

// zshChpwdHook, bashChpwdHook, fishChpwdHook, psChpwdHook close a gap the
// startup-only lines above cannot: shellDetectionLine/fishShellDetectionLine/
// psDetectionLine only run once per shell session, at startup, so `cd`-ing
// into a different repository inside an already-open terminal never
// re-triggers `detect`. Each hook caches the last-seen git top-level in a
// shell variable so a `cd` WITHIN the same repo (the common case) costs one
// cheap `git rev-parse --show-toplevel` and never actually invokes the CLI.
// Each is wrapped in its own installed-once guard (mirroring
// PROOFBOARD_DETECTED above) so `ensureLineInFile`'s whole-line substring
// dedup makes re-running install idempotent, and appending these as NEW
// entries in shellHookTargets' Lines slices leaves the legacy-migration
// path (legacyDetectionLines, ensureLineInFile, containsWholeLine)
// completely untouched.
func zshChpwdHook(bin string) string {
	return fmt.Sprintf(`if [ -z "$PROOFBOARD_CHPWD_INSTALLED" ]; then
PROOFBOARD_CHPWD_INSTALLED=1
typeset -g _proofboard_last_root=""
_proofboard_chpwd() {
  local root
  root=$(git rev-parse --show-toplevel 2>/dev/null)
  if [ -n "$root" ] && [ "$root" != "$_proofboard_last_root" ]; then
    _proofboard_last_root="$root"
    %s detect 2>/dev/null
  fi
}
autoload -Uz add-zsh-hook 2>/dev/null && add-zsh-hook chpwd _proofboard_chpwd
fi`, posixInvoke(bin))
}

func bashChpwdHook(bin string) string {
	return fmt.Sprintf(`if [ -z "$PROOFBOARD_CHPWD_INSTALLED" ]; then
PROOFBOARD_CHPWD_INSTALLED=1
_proofboard_last_root=""
_proofboard_chpwd() {
  local root
  root=$(git rev-parse --show-toplevel 2>/dev/null)
  if [ -n "$root" ] && [ "$root" != "$_proofboard_last_root" ]; then
    _proofboard_last_root="$root"
    %s detect 2>/dev/null
  fi
}
PROMPT_COMMAND="_proofboard_chpwd${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi`, posixInvoke(bin))
}

func fishChpwdHook(bin string) string {
	return fmt.Sprintf(`if not set -q PROOFBOARD_CHPWD_INSTALLED
set -g PROOFBOARD_CHPWD_INSTALLED 1
set -g _proofboard_last_root ""
function _proofboard_chpwd --on-variable PWD
    set -l root (git rev-parse --show-toplevel 2>/dev/null)
    if test -n "$root"; and test "$root" != "$_proofboard_last_root"
        set -g _proofboard_last_root $root
        %s detect 2>/dev/null
    end
end
end`, posixInvoke(bin))
}

// psChpwdHook: PowerShell has no native chpwd/PWD-change event; wrapping
// the prompt function is the standard idiom. This chains to whatever prompt
// function was already defined at install time (custom prompt, oh-my-posh,
// etc.) so it never clobbers an existing prompt.
func psChpwdHook(bin string) string {
	return fmt.Sprintf(`if (-not $env:PROOFBOARD_CHPWD_INSTALLED) {
    $env:PROOFBOARD_CHPWD_INSTALLED = "1"
    $global:_proofboardLastRoot = ""
    $global:_proofboardPrevPrompt = $function:prompt
    function global:prompt {
        try {
            $root = git rev-parse --show-toplevel 2>$null
            if ($root -and $root -ne $global:_proofboardLastRoot) {
                $global:_proofboardLastRoot = $root
                & "%s" detect 2>$null
            }
        } catch {}
        & $global:_proofboardPrevPrompt
    }
}`, bin)
}

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

// ensureShellDetectionHooks resolves the hook binary path from the
// currently-running process. This is correct for the self-healing,
// steady-state case — hook-maintain runs as part of an ordinary
// `proofboard <command>` invocation, so "whatever binary is running right
// now" and "the binary that should be in the hooks" are the same thing by
// definition, including after the CLI has been reinstalled to a new
// location. It is NOT correct during `install` itself: see
// ensureShellDetectionHooksForBinary.
func ensureShellDetectionHooks(ctx context.Context) (updated bool, inspected int, err error) {
	return ensureShellDetectionHooksForBinary(ctx, resolveHookBinaryPath())
}

// ensureShellDetectionHooksForBinary is ensureShellDetectionHooks with the
// hook binary path supplied explicitly, for the one caller where
// os.Executable() (inside resolveHookBinaryPath) would be wrong: `install`
// itself. At the moment installTo calls this, the *running* process is
// still whatever copied the executable into place — a temp download
// location, or a dev build run manually from a repo checkout — not the
// just-written destination at location.Executable. Writing hooks against
// os.Executable() there would point them at a path that may not even exist
// once the install script's temp files are cleaned up. installTo must pass
// its own resolved location.Executable explicitly instead of relying on
// the self-healing default.
func ensureShellDetectionHooksForBinary(ctx context.Context, bin string) (updated bool, inspected int, err error) {
	targets, err := shellHookTargets(bin)
	if err != nil {
		return false, 0, err
	}
	for _, target := range targets {
		inspected++
		if target.Path == "" {
			continue
		}
		changed, err := ensureWorkspaceDetectionBlock(target.Path, target.Lines)
		if err != nil {
			return false, inspected, err
		}
		if changed {
			updated = true
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

// ensureWorkspaceDetectionBlock makes sure every line in `lines` is present
// in the rc file at `path`, self-healing a stale block rather than trying
// to migrate each line individually. "Stale" covers more than the
// pre-header legacy formats ensureLineInFile already migrates in place: a
// block written by an earlier CLI version's template, or one pointing at a
// binary that has since moved (a reinstall to a new location, or simply a
// different resolveHookBinaryPath result), would otherwise sit there
// unmatched forever while a second, current block gets appended below it —
// duplicated text that still technically works (each is guarded against
// running twice) but never cleans itself up. Detecting staleness at the
// block level and replacing the whole thing avoids needing to hand-enumerate
// every historical line format, the way legacyDetectionLines otherwise
// requires.
//
// The common case — everything already present and current — costs exactly
// one file read and a handful of substring checks, same as calling
// ensureLineInFile directly for each line would have. Only a genuine
// mismatch pays for the strip-and-rewrite.
func ensureWorkspaceDetectionBlock(path string, lines []string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	allPresent := true
	for _, line := range lines {
		if !strings.Contains(string(content), line) {
			allPresent = false
			break
		}
	}
	if allPresent {
		return false, nil
	}
	// Something is missing or stale. If a block already exists under this
	// header, strip it first so the file never accumulates duplicates from
	// an old template or an old binary path sitting alongside the fresh one.
	// A header-less ultra-legacy line (predating this header entirely) has
	// no block to strip here; ensureLineInFile below still migrates it via
	// legacyDetectionLines, same as always.
	if strings.Contains(string(content), workspaceDetectionHeader) {
		if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
			return false, err
		}
	}
	for _, line := range lines {
		if _, _, err := ensureLineInFile(path, line); err != nil {
			return false, err
		}
	}
	return true, nil
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
// Ordered LONGEST FIRST, which is load-bearing rather than cosmetic.
// legacyShellDetectionLine is a complete line inside the two-line fish value,
// so checking it first replaced only fish's first line and left a bare
// `disown $last_pid` behind, a job-control builtin with no job to act on,
// which errored on every new fish shell. Longest-first makes the most
// specific value win.
var legacyDetectionLines = sortedLongestFirst([]string{
	legacyFishBackgroundedDetectionLine,
	legacyBackgroundedShellDetectionLine,
	legacyPSDetectionLine,
	legacyShellDetectionLine,
	legacyPlainShellDetectionLine,
	legacyPlainNoticeLine,
	legacyPlainPSDetectionLine,
	legacyPlainPSNoticeLine,
})

// sortedLongestFirst keeps the ordering guarantee above true by construction,
// so appending a new legacy value in the wrong place cannot reintroduce the
// fish bug.
func sortedLongestFirst(values []string) []string {
	out := append([]string{}, values...)
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
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
	if _, err := fmt.Fprintf(f, "\n%s\n%s\n", workspaceDetectionHeader, line); err != nil {
		return false, false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, false, nil
}

type detectHookTarget struct {
	Path  string
	Lines []string
}

func shellHookTargets(bin string) ([]detectHookTarget, error) {
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
			{Path: filepath.Join(homeDir, ".bashrc"), Lines: []string{shellDetectionLine(bin), noticeLine(bin), bashChpwdHook(bin)}},
			{Path: filepath.Join(homeDir, ".bash_profile"), Lines: []string{shellDetectionLine(bin), noticeLine(bin), bashChpwdHook(bin)}},
		}, nil
	case "zsh":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".zshrc"), Lines: []string{shellDetectionLine(bin), noticeLine(bin), zshChpwdHook(bin)}},
			{Path: filepath.Join(homeDir, ".zprofile"), Lines: []string{shellDetectionLine(bin), noticeLine(bin), zshChpwdHook(bin)}},
		}, nil
	case "fish":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".config", "fish", "config.fish"), Lines: []string{fishShellDetectionLine(bin), fishNoticeLine(bin), fishChpwdHook(bin)}},
		}, nil
	case "powershell", "pwsh":
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine(bin), psNoticeLine(bin), psChpwdHook(bin)}},
			{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine(bin), psNoticeLine(bin), psChpwdHook(bin)}},
		}, nil
	default:
		if os.Getenv("OS") == "Windows_NT" {
			return []detectHookTarget{
				{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine(bin), psNoticeLine(bin), psChpwdHook(bin)}},
				{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Lines: []string{psDetectionLine(bin), psNoticeLine(bin), psChpwdHook(bin)}},
			}, nil
		}
		// Plain POSIX sh (.profile fallback) has no chpwd/PROMPT_COMMAND
		// equivalent; startup-only detection remains correct here.
		return []detectHookTarget{
			{Path: filepath.Join(homeDir, ".profile"), Lines: []string{shellDetectionLine(bin), noticeLine(bin)}},
		}, nil
	}
}
