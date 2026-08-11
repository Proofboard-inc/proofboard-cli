//go:build windows

package commands

import (
	"context"
	"os/exec"
	"strings"
)

// isWorkspaceFocused reports whether the given workspace's IDE window is the
// one currently in the foreground. Uses a small inline P/Invoke snippet via
// PowerShell (GetForegroundWindow/GetWindowThreadProcessId/GetWindowText) —
// no extra Go dependency needed for a one-shot lookup like this.
// workspaceCount is the number of IDE workspaces the agent is currently
// tracking this tick (see discoverIDEWorkspaces); it bounds the fail-open
// fallback below.
func isWorkspaceFocused(ctx context.Context, workspace string, ideNames []string, workspaceCount int) bool {
	const script = `
$sig = '[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow(); [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId); [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, System.Text.StringBuilder lpString, int nMaxCount);'
Add-Type -MemberDefinition $sig -Name Win32Focus -Namespace ProofboardNative
$hwnd = [ProofboardNative.Win32Focus]::GetForegroundWindow()
$procId = 0
[ProofboardNative.Win32Focus]::GetWindowThreadProcessId($hwnd, [ref]$procId) | Out-Null
$sb = New-Object System.Text.StringBuilder 512
[ProofboardNative.Win32Focus]::GetWindowText($hwnd, $sb, 512) | Out-Null
$proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
Write-Output "$($proc.ProcessName)|$($sb.ToString())"
`
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		// Can't determine focus (PowerShell unavailable/failed). Fail open
		// (fire) only when this is the only workspace the agent is tracking
		// right now; with more than one tracked workspace we can't tell
		// whether THIS one is actually focused, so suppress instead.
		return workspaceCount <= 1
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	frontApp := parts[0]
	windowTitle := ""
	if len(parts) > 1 {
		windowTitle = strings.ToLower(parts[1])
	}
	if !matchesFrontmostIDEName(frontApp, ideNames) {
		return false
	}
	return windowTitleMatchesWorkspace(windowTitle, workspace)
}
