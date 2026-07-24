package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

const hookFileMode os.FileMode = 0o755

const (
	managedHookStart = "# >>> Proofboard Career Agent >>>"
	managedHookEnd   = "# <<< Proofboard Career Agent <<<"
	hookBackupSuffix = ".proofboard-original"
)

func Install(ctx context.Context, repo pbgit.Repo) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("install hooks: %w", err)
	}
	for _, name := range []string{"post-merge", "post-rewrite", "post-commit"} {
		if err := installHook(repo, name, hookSyncCommand(name)); err != nil {
			return err
		}
	}
	return nil
}

func hookSyncCommand(name string) string {
	if name == "post-commit" {
		return "proofboard sync --incremental --agent 2>/dev/null &"
	}
	return "proofboard sync --incremental --from-hook 2>/dev/null &"
}

func managedHookBlock(command string) string {
	return managedHookStart + "\n" + command + "\n" + managedHookEnd + "\n"
}

func installHook(repo pbgit.Repo, name, command string) error {
	path := filepath.Join(repo.Path, ".git", "hooks", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s hook: %w", name, err)
	}
	if strings.Contains(string(content), managedHookStart) || hasExactHookLine(content, command) {
		return makeHookExecutable(path, name)
	}
	if len(content) > 0 && !isShellHook(content) {
		return installWrappedHook(path, name, command)
	}
	if len(content) == 0 {
		content = []byte("#!/bin/sh\n")
	} else if content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}
	if len(content) > 0 && content[len(content)-1] == '\n' {
		content = append(content, '\n')
	}
	content = append(content, managedHookBlock(command)...)
	if err := os.WriteFile(path, content, hookFileMode); err != nil {
		return fmt.Errorf("write %s hook: %w", name, err)
	}
	return nil
}

func isShellHook(content []byte) bool {
	firstLine, _, _ := strings.Cut(string(content), "\n")
	firstLine = strings.ToLower(strings.TrimSpace(firstLine))
	if !strings.HasPrefix(firstLine, "#!") {
		return false
	}
	for _, shell := range []string{"/sh", "/bash", "/dash", "/ksh", "/zsh", " sh", " bash", " dash", " ksh", " zsh"} {
		if strings.Contains(firstLine, shell) {
			return true
		}
	}
	return false
}

func installWrappedHook(path, name, command string) error {
	backupPath := path + hookBackupSuffix
	if _, err := os.Lstat(backupPath); err == nil {
		return fmt.Errorf("install %s hook: preserved hook already exists at %s", name, backupPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect preserved %s hook: %w", name, err)
	}
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("preserve existing %s hook: %w", name, err)
	}
	wrapper := "#!/bin/sh\n" +
		managedHookStart + "\n" +
		"\"$0" + hookBackupSuffix + "\" \"$@\"\n" +
		"proofboard_hook_status=$?\n" +
		command + "\n" +
		"exit \"$proofboard_hook_status\"\n" +
		managedHookEnd + "\n"
	if err := os.WriteFile(path, []byte(wrapper), hookFileMode); err != nil {
		_ = os.Rename(backupPath, path)
		return fmt.Errorf("write wrapped %s hook: %w", name, err)
	}
	return nil
}

func hasExactHookLine(content []byte, command string) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == command {
			return true
		}
	}
	return false
}

func makeHookExecutable(path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s hook: %w", name, err)
	}
	if err := os.Chmod(path, info.Mode().Perm()|0o111); err != nil {
		return fmt.Errorf("make %s hook executable: %w", name, err)
	}
	return nil
}
