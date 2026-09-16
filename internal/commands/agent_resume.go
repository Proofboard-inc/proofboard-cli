package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// agentStoppedPath marks a deliberate `proofboard agent stop`, so that
// resuming the agent from a shell hook never undoes the developer's choice.
func agentStoppedPath(homeDir string) string {
	return filepath.Join(homeDir, ".proofboard", "agent.stopped")
}

// stopAgentByUser is `proofboard agent stop`. Internal restarts call stopAgent
// directly and leave no mark.
func stopAgentByUser(out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if err := stopAgent(out); err != nil {
		return err
	}
	path := agentStoppedPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Career Agent state directory: %w", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return fmt.Errorf("record Career Agent stop: %w", err)
	}
	return nil
}

func clearAgentStopped(homeDir string) error {
	if err := os.Remove(agentStoppedPath(homeDir)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear Career Agent stop: %w", err)
	}
	return nil
}

// resumeRegisteredAgent starts an installed agent that is not running and was
// not stopped on purpose. It runs from the shell-startup hook, which on a
// machine with no service manager and no desktop login is the only thing that
// runs after a reboot.
func resumeRegisteredAgent(homeDir string, start func() error) {
	if !agentRegistered(homeDir) {
		return
	}
	if _, err := os.Stat(agentStoppedPath(homeDir)); err == nil {
		return
	}
	if running, _ := agentRunning(homeDir); running {
		return
	}
	_ = start()
}

func startDetachedAgent() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Career Agent executable: %w", err)
	}
	cmd := exec.Command(execPath, "agent", "run")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return startDetachedCommand(cmd)
}
