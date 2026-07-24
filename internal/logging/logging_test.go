package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteSyncLog_CreationAndSafety(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proofboard-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	trigger := "manual"
	phase := "Phase 1: Ingest"
	outcome := "success"
	errMsg := ""

	err = WriteSyncLog(tempDir, repoHash, trigger, phase, outcome, errMsg)
	if err != nil {
		t.Fatalf("WriteSyncLog failed: %v", err)
	}

	logPath := filepath.Join(tempDir, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := strings.TrimSpace(string(data))
	parts := strings.Split(logContent, " — ")

	// Expected fields: timestamp, repoHash, trigger, phase, outcome
	if len(parts) != 5 {
		t.Errorf("expected 5 parts in log line, got %d. Content: %q", len(parts), logContent)
	}

	// Validate timestamp format
	_, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		t.Errorf("failed to parse timestamp %q as RFC3339: %v", parts[0], err)
	}

	if parts[1] != repoHash {
		t.Errorf("expected repo hash %q, got %q", repoHash, parts[1])
	}
	if parts[2] != trigger {
		t.Errorf("expected trigger %q, got %q", trigger, parts[2])
	}
	if parts[3] != phase {
		t.Errorf("expected phase %q, got %q", phase, parts[3])
	}
	if parts[4] != outcome {
		t.Errorf("expected outcome %q, got %q", outcome, parts[4])
	}

	// Test safety validation: make sure no emails or paths are present
	sensitiveSubjects := []string{"test@user.com", "src/main.go", "git commit message"}
	for _, sub := range sensitiveSubjects {
		if strings.Contains(logContent, sub) {
			t.Errorf("safety leak detected: log content contains sensitive data %q", sub)
		}
	}
}

func TestWriteSyncLog_WithError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proofboard-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	trigger := "hook"
	phase := "Phase 8: Transmission"
	outcome := "failure"
	errMsg := "network unreachable"

	err = WriteSyncLog(tempDir, repoHash, trigger, phase, outcome, errMsg)
	if err != nil {
		t.Fatalf("WriteSyncLog failed: %v", err)
	}

	logPath := filepath.Join(tempDir, ".proofboard", "sync.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := strings.TrimSpace(string(data))
	parts := strings.Split(logContent, " — ")

	// Expected fields: timestamp, repoHash, trigger, phase, outcome, error message
	if len(parts) != 6 {
		t.Errorf("expected 6 parts in log line, got %d. Content: %q", len(parts), logContent)
	}

	if parts[5] != redactedLogDetail {
		t.Errorf("expected redacted error detail %q, got %q", redactedLogDetail, parts[5])
	}
}

func TestWriteSyncLogSanitizesLegacySensitiveDetails(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, ".proofboard")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir log directory: %v", err)
	}
	logPath := filepath.Join(logDir, "sync.log")
	legacy := strings.Join([]string{
		`2026-07-24T12:00:00Z — HTTP POST — [REDACTED] — REQUEST {"repoName":"Secret Payments"} — RESPONSE (200 OK) {"organization":"Secret Employer"}`,
		`2026-07-24T12:00:01Z — repo-hash — agent — Phase 8 — failure — request failed for /home/ada/Secret Payments`,
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy log: %v", err)
	}

	if err := WriteSyncLog(tempDir, "repo-hash", "agent", "complete", "success", ""); err != nil {
		t.Fatalf("WriteSyncLog: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read sanitized log: %v", err)
	}
	for _, secret := range []string{"Secret Payments", "Secret Employer", "/home/ada"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("sanitized log retained %q: %s", secret, data)
		}
	}
	if !bytes.Contains(data, []byte(redactedLogDetail)) {
		t.Fatalf("sanitized log does not record redaction: %s", data)
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat sanitized log: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("sanitized log mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteSyncLog_Rotation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "proofboard-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, ".proofboard")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	logPath := filepath.Join(logDir, "sync.log")

	// Write 5MB of dummy data to sync.log
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("failed to create large dummy file: %v", err)
	}

	fiveMB := make([]byte, 5*1024*1024)
	for i := range fiveMB {
		fiveMB[i] = 'A'
	}
	_, err = file.Write(fiveMB)
	file.Close()
	if err != nil {
		t.Fatalf("failed to write 5MB: %v", err)
	}

	// Trigger WriteSyncLog which should rotate the file
	repoHash := "test-repo"
	err = WriteSyncLog(tempDir, repoHash, "manual", "start", "success", "")
	if err != nil {
		t.Fatalf("WriteSyncLog failed: %v", err)
	}

	// Verify sync.log.1 exists and has size of 5MB
	backupPath := logPath + ".1"
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file sync.log.1 does not exist: %v", err)
	}
	if backupInfo.Size() != 5*1024*1024 {
		t.Errorf("expected backup file size to be 5MB, got %d", backupInfo.Size())
	}

	// Verify sync.log now only has the new log entry
	newInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("new sync.log does not exist: %v", err)
	}
	if newInfo.Size() >= 5*1024*1024 {
		t.Errorf("expected new sync.log to be small, but size is %d", newInfo.Size())
	}

	newData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read new sync.log: %v", err)
	}
	if !bytes.Contains(newData, []byte(repoHash)) {
		t.Errorf("new sync.log does not contain new entry: %s", string(newData))
	}

	// Check that we can read rotated file data using io.LimitReader or similar if needed,
	// but simple existence and sizes are sufficient to prove size-based rotation.
}
