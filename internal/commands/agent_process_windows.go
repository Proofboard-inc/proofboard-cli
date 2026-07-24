//go:build windows

package commands

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func processExists(pid int) bool {
	filter := fmt.Sprintf("PID eq %d", pid)
	output, err := exec.Command("tasklist", "/FI", filter, "/FO", "CSV", "/NH").Output()
	return err == nil && strings.Contains(string(output), fmt.Sprintf(`"%d"`, pid))
}

func discoverIDEWorkspaces(ctx context.Context, configured []string) ([]string, error) {
	script := "Get-CimInstance Win32_Process | Select-Object Name,CommandLine | ConvertTo-Csv -NoTypeInformation"
	output, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil, fmt.Errorf("inspect IDE processes: %w", err)
	}
	ideNames := configuredIDENames(configured)
	records, err := csv.NewReader(strings.NewReader(string(output))).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("decode IDE process list: %w", err)
	}
	seen := make(map[string]bool)
	var workspaces []string
	ideRunning := false
	for _, record := range records[1:] {
		if len(record) < 2 || !matchesIDEProcess(record[0], ideNames) {
			continue
		}
		ideRunning = true
		for _, field := range splitCommandLine(record[1]) {
			addWorkspaceCandidate(ctx, field, seen, &workspaces)
		}
	}
	if ideRunning {
		discoverEditorStateWorkspaces(ctx, seen, &workspaces)
	}
	return workspaces, nil
}

func configuredIDENames(configured []string) []string {
	if len(configured) == 0 {
		return []string{"code.exe", "code-insiders.exe", "cursor.exe", "webstorm64.exe", "idea64.exe", "zed.exe", "sublime_text.exe", "vim.exe", "nvim.exe"}
	}
	return configured
}

func matchesIDEProcess(processName string, ideNames []string) bool {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(processName)), ".exe")
	for _, candidate := range ideNames {
		candidate = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(candidate)), ".exe")
		if name == candidate {
			return true
		}
	}
	return false
}
