package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RotationConfig struct {
	Path string
}

const (
	logPrivacyMigrationMarker = ".log-privacy-v2"
	redactedLogDetail         = "[details redacted]"
)

// LogEntry represents a structured log entry that complies with the NDA constraints.
type LogEntry struct {
	Timestamp     string
	RepoHash      string
	TriggerSource string
	Phase         string
	Outcome       string
	Error         string
}

// WriteSyncLog writes a clean and safe log line to the sync log file.
// It checks if the log file size exceeds 5MB. If it does, it rotates it
// (renames sync.log to sync.log.1, overwriting any previous .1, and starts a new sync.log).
// It ensures that only UTC timestamp, repo hash, trigger source, phase reached, outcome, and error message are written.
func WriteSyncLog(homeDir string, repoHash string, triggerSource string, phase string, outcome string, errMsg string) error {
	logDir := filepath.Join(homeDir, ".proofboard")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	if err := os.Chmod(logDir, 0o700); err != nil {
		return fmt.Errorf("secure log directory: %w", err)
	}
	if err := ensureLogPrivacyMigration(logDir); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "sync.log")

	// Check rotation
	info, err := os.Stat(logPath)
	if err == nil {
		// 5MB is 5 * 1024 * 1024 bytes
		if info.Size() >= 5*1024*1024 {
			backupPath := logPath + ".1"
			// Ignore remove error if it doesn't exist
			_ = os.Remove(backupPath)
			if err := os.Rename(logPath, backupPath); err != nil {
				return fmt.Errorf("rotate log file: %w", err)
			}
		}
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure log file: %w", err)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Clean inputs to ensure they contain no newlines
	cleanRepoHash := strings.ReplaceAll(repoHash, "\n", " ")
	cleanTrigger := strings.ReplaceAll(triggerSource, "\n", " ")
	cleanPhase := strings.ReplaceAll(phase, "\n", " ")
	cleanOutcome := strings.ReplaceAll(outcome, "\n", " ")
	cleanErrMsg := safeLogDetail(strings.ReplaceAll(errMsg, "\n", " "))

	var logLine string
	if cleanErrMsg != "" {
		logLine = fmt.Sprintf("%s — %s — %s — %s — %s — %s\n", timestamp, cleanRepoHash, cleanTrigger, cleanPhase, cleanOutcome, cleanErrMsg)
	} else {
		logLine = fmt.Sprintf("%s — %s — %s — %s — %s\n", timestamp, cleanRepoHash, cleanTrigger, cleanPhase, cleanOutcome)
	}

	if _, err := file.Write([]byte(logLine)); err != nil {
		return fmt.Errorf("write log line: %w", err)
	}
	return nil
}

func ensureLogPrivacyMigration(logDir string) error {
	markerPath := filepath.Join(logDir, logPrivacyMigrationMarker)
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect log privacy migration: %w", err)
	}
	for _, name := range []string{"sync.log", "sync.log.1"} {
		if err := sanitizeLegacyLog(filepath.Join(logDir, name)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(markerPath, []byte("v2\n"), 0o600); err != nil {
		return fmt.Errorf("write log privacy migration marker: %w", err)
	}
	if err := os.Chmod(markerPath, 0o600); err != nil {
		return fmt.Errorf("secure log privacy migration marker: %w", err)
	}
	return nil
}

func sanitizeLegacyLog(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy log: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	sanitized := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		if strings.Contains(line, " — HTTP ") && strings.Contains(line, " — REQUEST ") {
			changed = true
			continue
		}
		parts := strings.Split(line, " — ")
		if len(parts) > 5 {
			detail := strings.Join(parts[5:], " — ")
			safeDetail := safeLogDetail(detail)
			if safeDetail != detail {
				changed = true
			}
			line = strings.Join(append(parts[:5], safeDetail), " — ")
		}
		sanitized = append(sanitized, line)
	}
	if changed {
		output := strings.Join(sanitized, "\n")
		if output != "" {
			output += "\n"
		}
		if err := os.WriteFile(path, []byte(output), 0o600); err != nil {
			return fmt.Errorf("sanitize legacy log: %w", err)
		}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure legacy log: %w", err)
	}
	return nil
}

func safeLogDetail(detail string) string {
	if detail == "" {
		return ""
	}
	switch detail {
	case "workspace suppressed",
		"unlinked repo in hook",
		"not a production branch",
		"no new commits detected",
		"no new commits to sync",
		"trivial merge skipped",
		"no matching profile found",
		"workspace detection hook installed",
		"workspace detection hook already present":
		return detail
	default:
		return redactedLogDetail
	}
}
