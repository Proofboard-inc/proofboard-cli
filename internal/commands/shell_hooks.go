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
	"github.com/spf13/cobra"
)

const (
	legacyShellDetectionLine = "proofboard detect >/dev/null 2>&1 &"
	shellDetectionLine       = "(proofboard detect >/dev/null 2>&1 &)"
	fishShellDetectionLine   = "proofboard detect >/dev/null 2>&1 &\ndisown $last_pid"
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
		done, err := ensureLineInFile(target.Path, target.Line)
		if err != nil {
			return false, inspected, err
		}
		if done {
			updated = true
		}
	}
	return updated, inspected, nil
}

func ensureLineInFile(path string, line string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(content), line) {
		return false, nil
	}
	if strings.Contains(string(content), legacyShellDetectionLine) {
		updated := strings.ReplaceAll(string(content), legacyShellDetectionLine, line)
		mode := os.FileMode(0o644)
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path, []byte(updated), mode); err != nil {
			return false, fmt.Errorf("migrate %s: %w", path, err)
		}
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n# Proofboard Workspace Detection\n%s\n", line); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

type shellHookTarget struct {
	Path string
	Line string
}

func shellHookTargets() ([]shellHookTarget, error) {
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
		return []shellHookTarget{
			{Path: filepath.Join(homeDir, ".bashrc"), Line: shellDetectionLine},
			{Path: filepath.Join(homeDir, ".bash_profile"), Line: shellDetectionLine},
		}, nil
	case "zsh":
		return []shellHookTarget{
			{Path: filepath.Join(homeDir, ".zshrc"), Line: shellDetectionLine},
			{Path: filepath.Join(homeDir, ".zprofile"), Line: shellDetectionLine},
		}, nil
	case "fish":
		return []shellHookTarget{
			{Path: filepath.Join(homeDir, ".config", "fish", "config.fish"), Line: fishShellDetectionLine},
		}, nil
	case "powershell", "pwsh":
		line := "Start-Process -WindowStyle Hidden -FilePath proofboard -ArgumentList 'detect' | Out-Null"
		return []shellHookTarget{
			{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Line: line},
			{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Line: line},
		}, nil
	default:
		if os.Getenv("OS") == "Windows_NT" {
			line := "Start-Process -WindowStyle Hidden -FilePath proofboard -ArgumentList 'detect' | Out-Null"
			return []shellHookTarget{
				{Path: filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), Line: line},
				{Path: filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"), Line: line},
			}, nil
		}
		return []shellHookTarget{
			{Path: filepath.Join(homeDir, ".profile"), Line: shellDetectionLine},
		}, nil
	}
}
