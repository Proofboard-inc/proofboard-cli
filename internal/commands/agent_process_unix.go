//go:build linux || darwin

package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	return err == nil && process.Signal(syscall.Signal(0)) == nil
}

func discoverIDEWorkspaces(ctx context.Context, configured []string) ([]string, error) {
	output, err := exec.CommandContext(ctx, "ps", "-eo", "pid=,comm=,args=").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect IDE processes: %w", err)
	}
	ideNames := configuredIDENames(configured)
	seen := make(map[string]bool)
	var workspaces []string
	ideRunning := false
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := splitCommandLine(line)
		if len(fields) < 3 || !matchesIDEProcessLine(fields[1], line, ideNames) {
			continue
		}
		ideRunning = true
		pid, _ := strconv.Atoi(fields[0])
		candidates := fields[2:]
		if pid > 0 {
			if cwd, readErr := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd")); readErr == nil {
				candidates = append(candidates, cwd)
			}
		}
		for _, candidate := range candidates {
			addWorkspaceCandidate(ctx, candidate, seen, &workspaces)
		}
	}
	if ideRunning {
		discoverEditorStateWorkspaces(ctx, seen, &workspaces)
	}
	return workspaces, scanner.Err()
}

func configuredIDENames(configured []string) []string {
	if len(configured) == 0 {
		return []string{"code", "code-insiders", "cursor", "webstorm", "idea", "zed", "sublime_text", "vim", "nvim"}
	}
	return configured
}

func matchesIDEProcess(processName string, ideNames []string) bool {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(processName)), ".sh")
	for _, candidate := range ideNames {
		candidate = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(candidate)), ".sh")
		if name == candidate || strings.HasPrefix(name, candidate+"-") {
			return true
		}
	}
	return false
}

func matchesIDEProcessLine(processName, commandLine string, ideNames []string) bool {
	if matchesIDEProcess(processName, ideNames) {
		return true
	}
	lower := strings.ToLower(commandLine)
	for _, candidate := range ideNames {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "code" || candidate == "code-insiders" {
			if strings.Contains(lower, "visual studio code") || strings.Contains(lower, "/code ") {
				return true
			}
		}
		if candidate == "cursor" && (strings.Contains(lower, "cursor.app") || strings.Contains(lower, "/cursor ")) {
			return true
		}
	}
	return false
}
