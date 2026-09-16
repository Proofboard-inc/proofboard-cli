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
	"github.com/proofboard/proofboard/internal/model"
	"github.com/spf13/cobra"
)

const agentScanInterval = 15 * time.Second

type agentCommandActions struct {
	run     func(context.Context) error
	enable  func(io.Writer) error
	disable func(io.Writer) error
	start   func(context.Context, io.Writer) error
	stop    func(io.Writer) error
	status  func(context.Context, io.Writer) error
}

func newAgentCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return newAgentCommandWithActions(ctx, out, agentCommandActions{
		run:     runAgent,
		enable:  enableAgent,
		disable: uninstallAgentService,
		start:   startAgent,
		stop:    stopAgent,
		status:  printAgentStatus,
	})
}

func newAgentCommandWithActions(ctx context.Context, out io.Writer, actions agentCommandActions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage the local Proofboard Career Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return actions.status(ctx, out)
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:    "run",
			Short:  "Run the Career Agent in the foreground",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.run(ctx)
			},
		},
		&cobra.Command{
			Use:    "enable",
			Short:  "Register the Career Agent to start at sign-in",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.enable(out)
			},
		},
		&cobra.Command{
			Use:    "disable",
			Short:  "Remove the Career Agent sign-in registration",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.disable(out)
			},
		},
		&cobra.Command{
			Use:   "start",
			Short: "Start the Career Agent",
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.start(ctx, out)
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the Career Agent",
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.stop(out)
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show Career Agent status",
			RunE: func(cmd *cobra.Command, args []string) error {
				return actions.status(ctx, out)
			},
		},
	)
	return cmd
}

func enableAgent(out io.Writer) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Career Agent executable: %w", err)
	}
	return installAgentService(execPath, out)
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
	lastSyncLaunch := make(map[string]time.Time)
	if err := inspectIDEWorkspaces(ctx, runtime, lastSyncLaunch); err != nil {
		_ = logging.WriteSyncLog(runtime.homeDir, "career-agent", "agent", "workspace scan", "failure", err.Error())
	}
	// Executable auto-update rides this loop rather than a timer of its own:
	// the agent is already the thing that runs unattended, and
	// maybeAutoUpdateCLI is internally throttled to once a day, so calling it
	// every scan costs a state read and nothing else. It never blocks — the
	// installer is started detached and abandoned.
	maybeAutoUpdateCLI(ctx, runtime)

	ticker := time.NewTicker(agentScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := inspectIDEWorkspaces(ctx, runtime, lastSyncLaunch); err != nil {
				_ = logging.WriteSyncLog(runtime.homeDir, "career-agent", "agent", "workspace scan", "failure", err.Error())
			}
			maybeAutoUpdateCLI(ctx, runtime)
		}
	}
}

func inspectIDEWorkspaces(ctx context.Context, runtime runtimeContext, lastSyncLaunch map[string]time.Time) error {
	current, err := runtime.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("load agent state: %w", err)
	}
	workspaces, err := discoverIDEWorkspaces(ctx, current.IDEProcesses)
	if err != nil {
		return err
	}
	syncDiscoveredWorkspaces(ctx, runtime, workspaces, lastSyncLaunch, time.Now(), launchWorkspaceSync)
	return nil
}

// syncDiscoveredWorkspaces decides which of the workspaces open in an editor
// get a background sync launched, and launches them. It is separate from
// process discovery so the decision can be exercised without real editor
// processes.
func syncDiscoveredWorkspaces(
	ctx context.Context,
	runtime runtimeContext,
	workspaces []string,
	lastSyncLaunch map[string]time.Time,
	now time.Time,
	launch func(context.Context, string) error,
) {
	// Failure records live in state, written by the sync processes this
	// function launches. Read once per scan rather than per workspace.
	current, stateErr := runtime.state.Load(ctx)

	activeWorkspaces := make(map[string]bool, len(workspaces))
	for _, workspace := range workspaces {
		activeWorkspaces[workspace] = true
		result, inspectErr := detection.Inspect(ctx, runtime.homeDir, workspace, "career-agent")
		if inspectErr != nil {
			continue
		}
		activeWorkspaces[result.WorkspacePath] = true
		switch result.Action {
		case detection.ActionLink:
			// Intentionally silent here. "Project detected" is surfaced by the
			// shell startup hook instead (`proofboard detect`, backgrounded from
			// .zshrc/.zprofile/.bashrc; see shell_hooks.go), which prints
			// straight into the terminal the developer already has open
			// (printLinkDetected in detect.go). The shell hook is the sole
			// surface for this event; the agent only acts on ActionSync below,
			// which has no user-facing prompt of its own.
		case detection.ActionSync:
			// Throttle per repository, not per path. Editor helper processes
			// each report their own working directory, so one repository
			// arrives as its root plus whichever subfolders terminals and
			// extensions sit in. Keyed by path, every one of those launched a
			// sync of its own, and the same repository was transmitted two and
			// three times within a second.
			key := result.RepoHash
			if key == "" {
				key = result.RepoPath
			}
			activeWorkspaces[key] = true
			if now.Sub(lastSyncLaunch[key]) < time.Minute {
				continue
			}
			if stateErr == nil {
				if repo, ok := current.LinkedRepos[result.RepoHash]; ok && inTransmitBackoff(repo, now) {
					continue
				}
			}
			lastSyncLaunch[key] = now
			_ = launch(ctx, result.WorkspacePath)
		}
	}
	pruneInactiveWorkspaceSessions(lastSyncLaunch, activeWorkspaces)
}

// agentMaxTransmitBackoff caps how long the agent waits before retrying a
// repository whose transmissions keep failing. An hour keeps a server-side
// rejection from being resent all day while still recovering from a transient
// outage without anyone intervening.
const agentMaxTransmitBackoff = time.Hour

// transmitBackoff is the wait after the given number of consecutive failed
// transmissions: a minute, doubling each time, capped.
func transmitBackoff(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}
	backoff := time.Minute
	for i := 1; i < failures && backoff < agentMaxTransmitBackoff; i++ {
		backoff *= 2
	}
	if backoff > agentMaxTransmitBackoff {
		backoff = agentMaxTransmitBackoff
	}
	return backoff
}

// inTransmitBackoff reports whether the agent should leave a repository alone
// for now because its recent transmissions failed. The agent scans every
// fifteen seconds and used to relaunch a failing sync about once a minute for
// as long as the editor stayed open — 216 identical 500s for one repository in
// a day and a half. A sync the developer runs by hand is never subject to this.
func inTransmitBackoff(repo model.LinkedRepoState, now time.Time) bool {
	if repo.TransmitFailures <= 0 || repo.LastTransmitFailureAt.IsZero() {
		return false
	}
	return now.Sub(repo.LastTransmitFailureAt) < transmitBackoff(repo.TransmitFailures)
}

func pruneInactiveWorkspaceSessions(lastSyncLaunch map[string]time.Time, activeWorkspaces map[string]bool) {
	for workspace := range lastSyncLaunch {
		if !activeWorkspaces[workspace] {
			delete(lastSyncLaunch, workspace)
		}
	}
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
