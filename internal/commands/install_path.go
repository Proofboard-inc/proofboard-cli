package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
func ensureDirectoryOnPath(env installEnvironment, dir string, out io.Writer) error {
	if pathContainsDir(env.Getenv("PATH"), dir) {
		return nil
	}
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

// removeAllHeaderBlocks drops every occurrence of `header` found in the file
// at `path`, each together with the single line immediately following it.
// Every header this CLI writes (proofboardPathHeader,
// workspaceDetectionHeader, autocompletionHeader) is always followed by
// exactly one hook/command line — see appendMarkedLine and
// ensureLineInFile's "\n%s\n%s\n" writes and completion.go's matching
// "\n# Proofboard Autocompletion\n%s\n" write — so unlike removeMarkedLine
// (which matches one specific header+line pair) this removes every block
// under that header regardless of what the line itself says, which is what
// lets it clean up shell hooks written for a different shell/line than the
// one currently installed.
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
		if strings.TrimSpace(lines[index]) == strings.TrimSpace(header) {
			index++ // also drop the line immediately following the header
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
