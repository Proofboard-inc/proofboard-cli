package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/detection"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/spf13/cobra"
)

const agentScanInterval = 15 * time.Second

func newAgentCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage the local Proofboard Career Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printAgentStatus(ctx, out)
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:    "run",
			Short:  "Run the Career Agent in the foreground",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runAgent(ctx)
			},
		},
		&cobra.Command{
			Use:    "enable",
			Short:  "Register the Career Agent to start at sign-in",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				execPath, err := os.Executable()
				if err != nil {
					return fmt.Errorf("resolve Career Agent executable: %w", err)
				}
				return installAgentService(execPath, out)
			},
		},
		&cobra.Command{
			Use:    "disable",
			Short:  "Remove the Career Agent sign-in registration",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return uninstallAgentService(out)
			},
		},
		&cobra.Command{
			Use:   "start",
			Short: "Start the Career Agent",
			RunE: func(cmd *cobra.Command, args []string) error {
				return startAgent(ctx, out)
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the Career Agent",
			RunE: func(cmd *cobra.Command, args []string) error {
				return stopAgent(out)
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show Career Agent status",
			RunE: func(cmd *cobra.Command, args []string) error {
				return printAgentStatus(ctx, out)
			},
		},
	)
	return cmd
}

func agentPIDPath(homeDir string) string {
	return filepath.Join(homeDir, ".proofboard", "agent.pid")
}

func runAgent(ctx context.Context) error {
	runtime, err := loadRuntime(ctx)
	if err != nil {
		return fmt.Errorf("agent runtime: %w", err)
	}
	if err := claimAgentPID(runtime.homeDir); err != nil {
		return err
	}
	defer releaseAgentPID(runtime.homeDir, os.Getpid())
	if current, loadErr := runtime.state.Load(ctx); loadErr == nil {
		current.AuthReconnectPrompted = false
		_ = runtime.state.Save(ctx, current)
	}

	seenUnlinked := make(map[string]bool)
	lastSyncLaunch := make(map[string]time.Time)
	if err := inspectIDEWorkspaces(ctx, runtime, seenUnlinked, lastSyncLaunch); err != nil {
		_ = logging.WriteSyncLog(runtime.homeDir, "career-agent", "agent", "workspace scan", "failure", err.Error())
	}

	ticker := time.NewTicker(agentScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := inspectIDEWorkspaces(ctx, runtime, seenUnlinked, lastSyncLaunch); err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, "career-agent", "agent", "workspace scan", "failure", err.Error())
			}
		}
	}
}

func inspectIDEWorkspaces(ctx context.Context, runtime runtimeContext, seenUnlinked map[string]bool, lastSyncLaunch map[string]time.Time) error {
	current, err := runtime.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("load agent state: %w", err)
	}
	workspaces, err := discoverIDEWorkspaces(ctx, current.IDEProcesses)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		result, inspectErr := detection.Inspect(ctx, runtime.homeDir, workspace, "career-agent")
		if inspectErr != nil {
			continue
		}
		switch result.Action {
		case detection.ActionLink:
			if !seenUnlinked[result.WorkspacePath] {
				seenUnlinked[result.WorkspacePath] = true
				_ = launchWorkspaceNotification(ctx, runtime, result)
			}
		case detection.ActionSync:
			if time.Since(lastSyncLaunch[result.WorkspacePath]) >= time.Minute {
				lastSyncLaunch[result.WorkspacePath] = time.Now()
				_ = launchWorkspaceSync(ctx, result.WorkspacePath)
			}
		}
	}
	return nil
}

func startAgent(ctx context.Context, out io.Writer) error {
	runtime, err := loadRuntime(ctx)
	if err != nil {
		return fmt.Errorf("agent start: %w", err)
	}
	if running, _ := agentRunning(runtime.homeDir); running {
		_, err := fmt.Fprintln(out, "Proofboard Career Agent is already active.")
		return err
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Career Agent executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, execPath, "agent", "run")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := startDetachedCommand(cmd); err != nil {
		return fmt.Errorf("start Career Agent: %w", err)
	}
	_, err = fmt.Fprintln(out, "Proofboard Career Agent Active")
	return err
}

func stopAgent(out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	pid, err := readAgentPID(homeDir)
	if err != nil {
		if os.IsNotExist(err) {
			_, writeErr := fmt.Fprintln(out, "Proofboard Career Agent is not running.")
			return writeErr
		}
		return err
	}
	process, findErr := os.FindProcess(pid)
	if findErr == nil {
		findErr = process.Kill()
	}
	if findErr != nil && processExists(pid) {
		return fmt.Errorf("stop Career Agent: %w", findErr)
	}
	releaseAgentPID(homeDir, pid)
	_, err = fmt.Fprintln(out, "Proofboard Career Agent stopped.")
	return err
}

func printAgentStatus(ctx context.Context, out io.Writer) error {
	runtime, err := loadRuntime(ctx)
	if err != nil {
		return fmt.Errorf("agent status: %w", err)
	}
	running, _ := agentRunning(runtime.homeDir)
	status := "Stopped"
	if running {
		status = "Active"
	}
	_, err = fmt.Fprintf(out, "Proofboard Career Agent: %s\n", status)
	return err
}

func claimAgentPID(homeDir string) error {
	if running, pid := agentRunning(homeDir); running {
		return fmt.Errorf("cannot start Proofboard Career Agent: already running (pid %d)", pid)
	}
	path := agentPIDPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Career Agent state directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("write Career Agent pid: %w", err)
	}
	return nil
}

func agentRunning(homeDir string) (bool, int) {
	pid, err := readAgentPID(homeDir)
	if err != nil || !processExists(pid) {
		return false, 0
	}
	return true, pid
}

func readAgentPID(homeDir string) (int, error) {
	data, err := os.ReadFile(agentPIDPath(homeDir))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid Career Agent pid")
	}
	return pid, nil
}

func releaseAgentPID(homeDir string, expectedPID int) {
	pid, err := readAgentPID(homeDir)
	if err == nil && pid == expectedPID {
		_ = os.Remove(agentPIDPath(homeDir))
	}
}
