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

	// legacyBackgroundedShellDetectionLine was shellDetectionLine's value
	// until the fix below: it ran `detect` backgrounded with BOTH stdout and
	// stderr sent to /dev/null. That meant the "New repository detected"
	// prompt it prints on a match (see printLinkDetected in detect.go) was
	// discarded every single time — worse, detect ALSO records the prompt as
	// "shown" the moment it runs (see recordWorkspacePrompt), so this line
	// silently burned the one-time prompt for every workspace without the
	// developer ever having a chance to see it. `detect` only ever does fast,
	// local git plumbing (no network) with its own bounded timeout, so there
	// is no real latency reason to background or silence it — see
	// shellDetectionLine below, which now matches noticeLine's pattern
	// (synchronous, stderr-only suppression) instead.
	legacyBackgroundedShellDetectionLine = "(proofboard detect >/dev/null 2>&1 &)"
	legacyFishBackgroundedDetectionLine  = "proofboard detect >/dev/null 2>&1 &\ndisown $last_pid"

	shellDetectionLine     = "proofboard detect 2>/dev/null"
	fishShellDetectionLine = "proofboard detect 2>/dev/null"

	// noticeLine runs synchronously, same as shellDetectionLine above, so its
	// output is actually visible the moment a terminal starts up — the same
	// subtle "one line on shell startup" pattern as a venv auto-activation
	// hook. Only stderr is suppressed; a slow network is already bounded by a
	// short timeout inside the command itself.
	noticeLine     = "proofboard notices 2>/dev/null"
	fishNoticeLine = "proofboard notices 2>/dev/null"
	psNoticeLine   = "proofboard notices 2>$null"

	legacyPSDetectionLine = "Start-Process -WindowStyle Hidden -FilePath proofboard -ArgumentList 'detect' | Out-Null"
	psDetectionLine       = "proofboard detect 2>$null"
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
			done, err := ensureLineInFile(target.Path, line)
			if err != nil {
				return false, inspected, err
			}
			if done {
				updated = true
			}
		}
	}
	return updated, inspected, nil
}

// legacyDetectionLines are every prior value of the shell startup detection
// line, oldest first — checked in ensureLineInFile so an existing install
// gets migrated onto the current line instead of ending up with two (or,
// after this fix, silently keeping the still-broken backgrounded/silenced
// one forever since it never matches "line already present").
var legacyDetectionLines = []string{
	legacyShellDetectionLine,
	legacyBackgroundedShellDetectionLine,
	legacyFishBackgroundedDetectionLine,
	legacyPSDetectionLine,
}

func ensureLineInFile(path string, line string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(content), line) {
		return false, nil
	}
	for _, legacy := range legacyDetectionLines {
		if !strings.Contains(string(content), legacy) {
			continue
		}
		updated := strings.ReplaceAll(string(content), legacy, line)
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
