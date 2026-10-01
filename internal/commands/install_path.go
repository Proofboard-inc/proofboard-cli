package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const proofboardPathHeader = "# Proofboard Career Agent"

// workspaceDetectionHeader and autocompletionHeader mirror the literal header
// comments written by shell_hooks.go's ensureLineInFile and completion.go's
// auto-install path, respectively. Kept here (next to proofboardPathHeader)
// so uninstall's header-block sweep has a single place enumerating every
// header this CLI ever writes into a shell rc file.
const workspaceDetectionHeader = "# Proofboard Workspace Detection"
const autocompletionHeader = "# Proofboard Autocompletion"

// allShellHookHeaders lists every header comment this CLI writes into a shell
// rc file, across install (PATH), shell hook maintenance (workspace
// detection), and completion auto-install. Uninstall sweeps all of them so a
// full uninstall leaves no dangling references to a removed binary.
var allShellHookHeaders = []string{proofboardPathHeader, workspaceDetectionHeader, autocompletionHeader}

// rcFileCandidates enumerates every shell rc/profile file any code path in
// this CLI might have written a header block into, regardless of the user's
// *current* $SHELL — a user may have switched shells since installing, and a
// stale hook in an old shell's rc file is exactly the kind of dangling
// reference uninstall is supposed to remove. Scanning a superset is safe:
// removeAllHeaderBlocks is a no-op on a file with no matching header.
func rcFileCandidates(env installEnvironment) []string {
	return []string{
		filepath.Join(env.HomeDir, ".bashrc"),
		filepath.Join(env.HomeDir, ".bash_profile"),
		filepath.Join(env.HomeDir, ".zshrc"),
		filepath.Join(env.HomeDir, ".zprofile"),
		filepath.Join(env.HomeDir, ".profile"),
		filepath.Join(env.HomeDir, ".config", "fish", "config.fish"),
		filepath.Join(env.HomeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(env.HomeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}
}

// removeShellHookBlocks strips every block written under any header in
// allShellHookHeaders from every candidate rc file. Errors reading/writing an
// individual file are ignored the same way removeDirectoryFromPath already
// ignores them elsewhere in uninstall: a missing or unwritable rc file must
// never fail the overall uninstall.
func removeShellHookBlocks(env installEnvironment) {
	for _, path := range rcFileCandidates(env) {
		for _, header := range allShellHookHeaders {
			_ = removeAllHeaderBlocks(path, header)
		}
	}
}

type shellHookTarget struct {
	Path string
	Line string
}

// ensureDirectoryOnPath makes a per-user installation directory reachable as a
// plain `proofboard` command. Nothing here needs administrator access: on
// Windows only the per-user PATH is edited, and elsewhere the user's own shell
// profile is.
//
// This deliberately does NOT early-return just because dir is already on the
// *current process's* inherited PATH: that reflects whatever shell happened
// to invoke the installer, not what is durably persisted in any rc file. A
// user who previously exported the directory by hand in one open terminal
// (or ran install/uninstall repeatedly in the same session) would otherwise
// cause every OTHER terminal, IDE-integrated shell, and non-login shell to
// silently never get `proofboard` on PATH — and since every shell hook line
// this CLI writes redirects stderr to /dev/null, that failure is completely
// invisible. appendMarkedLine/ensureShellProfilePath already dedupe against
// the rc files' actual on-disk content, so calling this unconditionally is
// safe and idempotent.
func ensureDirectoryOnPath(env installEnvironment, dir string, out io.Writer) error {
	if env.GOOS == "windows" {
		return ensureWindowsUserPath(dir, out)
	}
	return ensureShellProfilePath(env, dir, out)
}

func ensureWindowsUserPath(dir string, out io.Writer) error {
	script := fmt.Sprintf(`$directory = %q
$current = [Environment]::GetEnvironmentVariable('Path', 'User')
if ([string]::IsNullOrEmpty($current)) { $current = '' }
$entries = $current -split ';' | Where-Object { $_ -ne '' }
if ($entries -notcontains $directory) {
    $updated = (@($entries) + $directory) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
}`, dir)
	command := exec.Command("powershell", "-NoProfile", "-Command", script)
	if output, err := command.CombinedOutput(); err != nil {
		_, _ = fmt.Fprintf(out, "Add %s to your PATH to use the proofboard command: %v\n", dir, err)
		return nil
	} else if len(output) > 0 {
		_, _ = fmt.Fprint(out, string(output))
	}
	_, _ = fmt.Fprintf(out, "Added %s to your PATH. Open a new terminal to use the proofboard command.\n", dir)
	return nil
}

func ensureShellProfilePath(env installEnvironment, dir string, out io.Writer) error {
	targets := shellProfilePathTargets(env, dir)
	if len(targets) == 0 {
		_, _ = fmt.Fprintf(out, "Add %s to your PATH to use the proofboard command.\n", dir)
		return nil
	}

	updated := []string{}
	for _, target := range targets {
		changed, err := appendMarkedLine(target.Path, proofboardPathHeader, target.Line)
		if err != nil {
			_, _ = fmt.Fprintf(out, "Add %s to your PATH to use the proofboard command: %v\n", dir, err)
			return nil
		}
		if changed {
			updated = append(updated, target.Path)
		}
	}
	if len(updated) > 0 {
		_, _ = fmt.Fprintf(out, "Added %s to your PATH in %s. Open a new terminal to use the proofboard command.\n",
			dir, strings.Join(updated, ", "))
	}
	return nil
}

func shellProfilePathTargets(env installEnvironment, dir string) []shellHookTarget {
	shell := strings.ToLower(filepath.Base(env.Getenv("SHELL")))
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)

	switch shell {
	case "bash":
		return []shellHookTarget{
			{Path: filepath.Join(env.HomeDir, ".bashrc"), Line: exportLine},
			{Path: filepath.Join(env.HomeDir, ".bash_profile"), Line: exportLine},
		}
	case "zsh":
		return []shellHookTarget{
			{Path: filepath.Join(env.HomeDir, ".zshrc"), Line: exportLine},
			{Path: filepath.Join(env.HomeDir, ".zprofile"), Line: exportLine},
		}
	case "fish":
		return []shellHookTarget{
			{Path: filepath.Join(env.HomeDir, ".config", "fish", "config.fish"), Line: fmt.Sprintf("fish_add_path %q", dir)},
		}
	default:
		return []shellHookTarget{
			{Path: filepath.Join(env.HomeDir, ".profile"), Line: exportLine},
		}
	}
}

func appendMarkedLine(path, header, line string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(content), line) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "\n%s\n%s\n", header, line); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// removeMarkedLine drops a previously appended PATH entry and its header.
func removeMarkedLine(path, header, line string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !strings.Contains(string(content), line) {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == strings.TrimSpace(header) &&
			index+1 < len(lines) && strings.TrimSpace(lines[index+1]) == strings.TrimSpace(line) {
			index++
			continue
		}
		if strings.TrimSpace(lines[index]) == strings.TrimSpace(line) {
			continue
		}
		kept = append(kept, lines[index])
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// hookBinaryPlaceholder stands in for the binary path inside the block-body
// patterns below, so one pattern covers a hook written against any install
// location. It cannot occur in a real path: the leading NUL byte is not a
// legal character in a filename on any platform this CLI supports.
const hookBinaryPlaceholder = "\x00proofboard-binary\x00"

// knownBlockBodies maps every header this CLI writes into a shell rc file to
// each block body that can appear under it — the current templates for every
// shell, the PATH export for every shell, and every legacy line still
// recognised by ensureLineInFile's migration path. Both are generated from
// the very same constants/functions the writes use, so this table cannot drift
// from what a given CLI version actually puts in a file.
var knownBlockBodies = map[string][]string{
	proofboardPathHeader: {
		`export PATH="` + hookBinaryPlaceholder + `:$PATH"`,
		`fish_add_path "` + hookBinaryPlaceholder + `"`,
	},
	workspaceDetectionHeader: {
		shellDetectionLine(hookBinaryPlaceholder),
		noticeLine(hookBinaryPlaceholder),
		bashChpwdHook(hookBinaryPlaceholder),
		zshChpwdHook(hookBinaryPlaceholder),
		fishShellDetectionLine(hookBinaryPlaceholder),
		fishNoticeLine(hookBinaryPlaceholder),
		fishChpwdHook(hookBinaryPlaceholder),
		psDetectionLine(hookBinaryPlaceholder),
		psNoticeLine(hookBinaryPlaceholder),
		psChpwdHook(hookBinaryPlaceholder),
		legacyShellDetectionLine,
		legacyBackgroundedShellDetectionLine,
		legacyFishBackgroundedDetectionLine,
		legacyPSDetectionLine,
		legacyPlainShellDetectionLine,
		legacyPlainNoticeLine,
		legacyPlainPSDetectionLine,
		legacyPlainPSNoticeLine,
	},
	autocompletionHeader: {
		`source <(` + hookBinaryPlaceholder + ` completion bash)`,
		`source <(` + hookBinaryPlaceholder + ` completion zsh)`,
	},
}

// knownBlockBodyPatterns compiles knownBlockBodies into anchored patterns: a
// candidate body matches only if it is exactly one of them, never merely a
// substring. Anchoring matters — several bodies contain a line that is on its
// own a complete body of another hook (e.g. the bare
// "PROOFBOARD_CHPWD_INSTALLED=1" inside zshChpwdHook), and an unanchored
// match would consume one line of a multi-line hook and orphan the rest.
var knownBlockBodyPatterns = func() map[string][]*regexp.Regexp {
	compiled := make(map[string][]*regexp.Regexp, len(knownBlockBodies))
	for header, bodies := range knownBlockBodies {
		patterns := make([]*regexp.Regexp, 0, len(bodies))
		for _, body := range bodies {
			parts := strings.Split(body, hookBinaryPlaceholder)
			quoted := make([]string, 0, len(parts))
			for _, part := range parts {
				quoted = append(quoted, regexp.QuoteMeta(part))
			}
			// The path is always written inside quotes (posixInvoke, the
			// PowerShell "& \"%s\"" call operator, and the PATH export all
			// quote it), so a run of non-quote characters is exactly the set
			// of valid paths here.
			patterns = append(patterns, regexp.MustCompile("^(?:"+strings.Join(quoted, `[^"]*`)+")$"))
		}
		compiled[header] = patterns
	}
	return compiled
}()

// maxKnownBlockBodyLines is the longest known body in lines, so the search in
// blockBodyLineCount is bounded by it instead of by the length of the file.
var maxKnownBlockBodyLines = func() int {
	longest := 1
	for _, bodies := range knownBlockBodies {
		for _, body := range bodies {
			if lines := strings.Count(body, "\n") + 1; lines > longest {
				longest = lines
			}
		}
	}
	return longest
}()

// blockBodyLineCount returns how many of the lines following `header` belong
// to the block that header introduces. It stops at the first line that is not
// part of a known body, so anything a user wrote immediately after a block —
// with no blank line in between, which is exactly what appending to a rc file
// produces — survives untouched.
func blockBodyLineCount(header string, following []string) int {
	// Longest match first: one legacy body is a prefix of another (the fish
	// "detect &" + "disown $last_pid" pair starts with the plain backgrounded
	// detect line), and matching the shorter one would leave the second line
	// orphaned. Every write is one header plus exactly one body, so the
	// longest exact match is always the whole block.
	limit := len(following)
	if limit > maxKnownBlockBodyLines {
		limit = maxKnownBlockBodyLines
	}
	for count := limit; count >= 1; count-- {
		candidate := strings.Join(following[:count], "\n")
		for _, pattern := range knownBlockBodyPatterns[header] {
			if pattern.MatchString(candidate) {
				return count
			}
		}
	}
	// No known body matches: a hand-edited block, or a format from a version
	// old enough that its constants are no longer in this build. Remove only
	// what is unmistakably Proofboard's — a run of lines naming the CLI — so a
	// stale block is still cleaned up without reaching into the user's own
	// configuration.
	count := 0
	for _, line := range following {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		for _, other := range allShellHookHeaders {
			if trimmed == other {
				return count
			}
		}
		if !strings.Contains(strings.ToLower(trimmed), "proofboard") {
			break
		}
		count++
	}
	// A body that opened a shell construct without matching a known pattern
	// can leave a bare closing keyword behind once its header and its
	// Proofboard lines are gone. A bare "fi"/"}" cannot be valid content on
	// its own, so consuming it repairs the file rather than leaving the parse
	// error this function exists to prevent.
	for count < len(following) && isBareShellCloser(strings.TrimSpace(following[count])) {
		count++
	}
	return count
}

func isBareShellCloser(line string) bool {
	switch line {
	case "fi", "}", "done", "esac", "end":
		return true
	}
	return false
}

// removeAllHeaderBlocks drops every occurrence of `header` found in the file
// at `path`, each together with the block body written under it. Every header
// this CLI writes (proofboardPathHeader, workspaceDetectionHeader,
// autocompletionHeader) is written by appendMarkedLine / ensureLineInFile's
// "\n%s\n%s\n" format, or completion.go's matching "\n# Proofboard
// Autocompletion\n%s\n" — the %s "line" is not always a single line (see
// zshChpwdHook/bashChpwdHook/fishChpwdHook/psChpwdHook in shell_hooks.go,
// which are each several lines of an if/fi- or function-wrapped hook).
//
// blockBodyLineCount decides how much of the file belongs to each block. Two
// earlier implementations got this wrong in opposite directions: removing a
// fixed single line after the header left the body and closing "fi" of a
// multi-line hook orphaned, which broke zsh startup with a parse error; and
// reading to the next blank line fixed that but silently deleted whatever the
// user had appended directly after a block with no blank line in between —
// trading a broken rc file for lost configuration, in the one code path whose
// entire job is to leave rc files intact. Matching the block body against the
// templates this CLI actually writes removes exactly the block and nothing
// else.
func removeAllHeaderBlocks(path, header string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !strings.Contains(string(content), header) {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != strings.TrimSpace(header) {
			kept = append(kept, lines[index])
			continue
		}
		index += blockBodyLineCount(header, lines[index+1:])
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
