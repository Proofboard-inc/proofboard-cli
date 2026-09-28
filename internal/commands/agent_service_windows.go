//go:build windows

package commands

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const agentTaskName = "Proofboard Career Agent"

func installAgentService(executable string, out io.Writer) error {
	if err := registerProtocolHandler(executable); err != nil {
		return fmt.Errorf("register Career Agent notification actions: %w", err)
	}
	_ = exec.Command("schtasks", "/End", "/TN", agentTaskName).Run()
	if err := stopAgent(io.Discard); err != nil {
		return fmt.Errorf("stop existing Career Agent: %w", err)
	}
	command := fmt.Sprintf(`"%s" agent run`, executable)
	if output, err := exec.Command("schtasks", "/Create", "/TN", agentTaskName, "/TR", command, "/SC", "ONLOGON", "/F").CombinedOutput(); err != nil {
		return fmt.Errorf("register Career Agent scheduled task: %w: %s", err, output)
	}
	if output, err := exec.Command("schtasks", "/Run", "/TN", agentTaskName).CombinedOutput(); err != nil {
		return fmt.Errorf("start Career Agent scheduled task: %w: %s", err, output)
	}
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent registered to start when you sign in.")
	return nil
}

func uninstallAgentService(out io.Writer) error {
	_ = exec.Command("schtasks", "/End", "/TN", agentTaskName).Run()
	if output, err := exec.Command("schtasks", "/Delete", "/TN", agentTaskName, "/F").CombinedOutput(); err != nil && !taskNotFound(output) {
		return fmt.Errorf("remove Career Agent scheduled task: %w: %s", err, output)
	}
	_ = stopAgent(io.Discard)
	_ = unregisterProtocolHandler()
	_, _ = fmt.Fprintln(out, "Proofboard Career Agent background service removed.")
	return nil
}

// agentRegistered is false here: the service manager this platform registers
// with starts the agent again after a reboot, so a shell hook has nothing to
// resume.
func agentRegistered(homeDir string) bool {
	return false
}

// taskNotFound reports whether schtasks failed because the task was already
// absent, so uninstall can treat "nothing to remove" the same way the
// darwin/linux implementations do (os.IsNotExist), rather than as an error.
func taskNotFound(output []byte) bool {
	return strings.Contains(strings.ToLower(string(output)), "cannot find the file specified")
}
