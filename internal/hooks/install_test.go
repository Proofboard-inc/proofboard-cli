package hooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func TestInstallAndUninstallPreserveExistingHooks(t *testing.T) {
	repo := testRepo(t)
	hookPath := filepath.Join(repo.Path, ".git", "hooks", "post-commit")
	existing := "#!/bin/sh\necho user-hook\n"
	if err := os.WriteFile(hookPath, []byte(existing), 0o750); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(context.Background(), repo); err != nil {
		t.Fatalf("Install first pass: %v", err)
	}
	if err := Install(context.Background(), repo); err != nil {
		t.Fatalf("Install second pass: %v", err)
	}
	installed, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read installed hook: %v", err)
	}
	if strings.Count(string(installed), "echo user-hook") != 1 {
		t.Fatalf("existing hook was not preserved exactly once: %s", installed)
	}
	if strings.Count(string(installed), managedHookStart) != 1 ||
		strings.Count(string(installed), hookSyncCommand("post-commit")) != 1 {
		t.Fatalf("Proofboard hook was not installed idempotently: %s", installed)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(hookPath)
		if err != nil {
			t.Fatalf("stat installed hook: %v", err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("installed hook is not executable: %v", info.Mode().Perm())
		}
	}

	if err := Uninstall(context.Background(), repo); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	remaining, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("existing hook was removed: %v", err)
	}
	if !strings.Contains(string(remaining), "echo user-hook") {
		t.Fatalf("existing hook content was removed: %s", remaining)
	}
	if strings.Contains(string(remaining), "Proofboard") || strings.Contains(string(remaining), "proofboard sync") {
		t.Fatalf("Proofboard block remained after uninstall: %s", remaining)
	}
}

func TestUninstallRemovesProofboardOnlyAndLegacyHooks(t *testing.T) {
	tests := map[string]string{
		"managed": "#!/bin/sh\n" + managedHookBlock(hookSyncCommand("post-merge")),
		"legacy":  "#!/bin/sh\n" + hookSyncCommand("post-merge") + "\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			repo := testRepo(t)
			hookPath := filepath.Join(repo.Path, ".git", "hooks", "post-merge")
			if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
				t.Fatalf("write hook: %v", err)
			}
			if err := Uninstall(context.Background(), repo); err != nil {
				t.Fatalf("Uninstall: %v", err)
			}
			if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
				t.Fatalf("Proofboard-only hook remains: %v", err)
			}
		})
	}
}

func TestInstallWrapsAndRestoresNonShellHook(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required to exercise a non-shell Git hook")
	}
	repo := testRepo(t)
	hookPath := filepath.Join(repo.Path, ".git", "hooks", "post-commit")
	existing := "#!/usr/bin/env python3\nfrom pathlib import Path\nPath('user-hook-ran').write_text('yes')\n"
	if err := os.WriteFile(hookPath, []byte(existing), 0o750); err != nil {
		t.Fatalf("write Python hook: %v", err)
	}

	if err := Install(context.Background(), repo); err != nil {
		t.Fatalf("Install: %v", err)
	}
	wrapped, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}
	if !strings.Contains(string(wrapped), managedHookStart) ||
		!strings.Contains(string(wrapped), hookBackupSuffix) {
		t.Fatalf("non-shell hook was not safely wrapped: %s", wrapped)
	}
	preserved, err := os.ReadFile(hookPath + hookBackupSuffix)
	if err != nil {
		t.Fatalf("read preserved hook: %v", err)
	}
	if string(preserved) != existing {
		t.Fatalf("preserved hook changed:\n%s", preserved)
	}

	command := exec.Command(hookPath)
	command.Dir = repo.Path
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run wrapped hook: %v: %s", err, output)
	}
	if data, err := os.ReadFile(filepath.Join(repo.Path, "user-hook-ran")); err != nil || string(data) != "yes" {
		t.Fatalf("original hook did not run through wrapper: data=%q err=%v", data, err)
	}

	if err := Uninstall(context.Background(), repo); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	restored, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read restored hook: %v", err)
	}
	if string(restored) != existing {
		t.Fatalf("restored hook changed:\n%s", restored)
	}
	if _, err := os.Lstat(hookPath + hookBackupSuffix); !os.IsNotExist(err) {
		t.Fatalf("preserved hook sidecar remains after restore: %v", err)
	}
}

func testRepo(t *testing.T) pbgit.Repo {
	t.Helper()
	path := t.TempDir()
	if err := os.MkdirAll(filepath.Join(path, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks directory: %v", err)
	}
	return pbgit.Repo{Path: path}
}
