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
