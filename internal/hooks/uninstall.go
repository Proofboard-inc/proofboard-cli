package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func Uninstall(ctx context.Context, repo pbgit.Repo) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("uninstall hooks: %w", err)
	}
	for _, hook := range []string{"post-commit", "post-merge", "post-rewrite"} {
		if err := uninstallHook(repo, hook, hookSyncCommand(hook)); err != nil {
			return err
		}
	}
	return nil
}

func uninstallHook(repo pbgit.Repo, name, command string) error {
	path := filepath.Join(repo.Path, ".git", "hooks", name)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s hook: %w", name, err)
	}
	backupPath := path + hookBackupSuffix
	if strings.Contains(string(content), managedHookStart) {
		if _, backupErr := os.Lstat(backupPath); backupErr == nil {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove wrapped %s hook: %w", name, err)
			}
			if err := os.Rename(backupPath, path); err != nil {
				return fmt.Errorf("restore preserved %s hook: %w", name, err)
			}
			return nil
		} else if !os.IsNotExist(backupErr) {
			return fmt.Errorf("inspect preserved %s hook: %w", name, backupErr)
		}
	}
	updated := removeManagedHookBlock(string(content))
	updated = removeExactHookLine(updated, command)
	if hookIsEmpty(updated) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s hook: %w", name, err)
		}
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s hook: %w", name, err)
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return fmt.Errorf("update %s hook: %w", name, err)
	}
	return nil
}

func removeManagedHookBlock(content string) string {
	for {
		start := strings.Index(content, managedHookStart)
		if start < 0 {
			return content
		}
		endOffset := strings.Index(content[start:], managedHookEnd)
		if endOffset < 0 {
			return content
		}
		end := start + endOffset + len(managedHookEnd)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		if start > 0 && content[start-1] == '\n' {
			start--
		}
		content = content[:start] + content[end:]
	}
}

func removeExactHookLine(content, command string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != command {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func hookIsEmpty(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && trimmed != "#!/bin/sh" && trimmed != "#!/usr/bin/env sh" {
			return false
		}
	}
	return true
}
