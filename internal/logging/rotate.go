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

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Clean inputs to ensure they contain no newlines
	cleanRepoHash := strings.ReplaceAll(repoHash, "\n", " ")
	cleanTrigger := strings.ReplaceAll(triggerSource, "\n", " ")
	cleanPhase := strings.ReplaceAll(phase, "\n", " ")
	cleanOutcome := strings.ReplaceAll(outcome, "\n", " ")
	cleanErrMsg := strings.ReplaceAll(errMsg, "\n", " ")

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
