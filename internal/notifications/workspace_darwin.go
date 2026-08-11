//go:build darwin

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// showWorkspaceAction prefers a real Notification Center banner via the
// optional `terminal-notifier` tool (https://github.com/julienXX/terminal-notifier,
// `brew install terminal-notifier`) — corner-positioned and non-blocking,
// unlike the AppleScript `display dialog` fallback below, which is always
// centered and modal with no way to reposition it. Falls back automatically
// to the dialog whenever terminal-notifier isn't on PATH, or fails for any
// reason, so a notification always shows either way.
func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	if _, err := exec.LookPath("terminal-notifier"); err == nil {
		if notifierErr := showWorkspaceActionViaTerminalNotifier(ctx, action); notifierErr == nil {
			return nil
		}
	}
	return showWorkspaceActionViaDialog(ctx, action)
}

// terminalNotifierResult mirrors terminal-notifier's `-json` output shape.
type terminalNotifierResult struct {
	ActivationType  string `json:"activationType"`
	ActivationValue string `json:"activationValue"`
}

func showWorkspaceActionViaTerminalNotifier(ctx context.Context, action WorkspaceAction) error {
	title, body, primary, secondary, tertiary := workspaceActionLabels(action.Kind)
	primaryKey, secondaryKey, tertiaryKey := workspaceActionKeys(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s — %s", body, action.RepoName)
	}

	actions := []string{primary}
	if secondary != "" {
		actions = append(actions, secondary)
	}
	if tertiary != "" {
		actions = append(actions, tertiary)
	}

	// -wait blocks until the user interacts (or the notification expires) and
	// -json prints the result as structured JSON on stdout — this process is
	// already a detached, dedicated `proofboard notify` subprocess (see
	// notify.go/agent.go), so blocking here is the same safe pattern already
	// used by the dialog fallback's own blocking `giving up after 120`.
	args := []string{
		"-title", title,
		"-message", body,
		"-actions", strings.Join(actions, ","),
		"-wait",
		"-json",
	}
	cmd := exec.CommandContext(ctx, "terminal-notifier", args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("terminal-notifier: %w", err)
	}

	var result terminalNotifierResult
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("parse terminal-notifier output: %w", err)
	}
	kind, ok := resolveTerminalNotifierActivation(result, activationLabels{
		primary, primaryKey, secondary, secondaryKey, tertiary, tertiaryKey,
	})
	if !ok {
		// Dismissed, timed out, or the notification body was clicked without
		// picking a specific action from the dropdown — nothing to activate.
		return nil
	}
	return ActivateWorkspaceAction(ctx, kind, action.Workspace, action.Target)
}

type activationLabels struct {
	primary      string
	primaryKey   string
	secondary    string
	secondaryKey string
	tertiary     string
	tertiaryKey  string
}

// resolveTerminalNotifierActivation maps a terminal-notifier -json result
// back to the action key ("sync", "review", "publish", "ignore", ...) it
// corresponds to. Pulled out as a pure function so the mapping logic is
// unit-testable without invoking the actual terminal-notifier binary.
func resolveTerminalNotifierActivation(result terminalNotifierResult, l activationLabels) (kind string, ok bool) {
	if result.ActivationType != "actionClicked" {
		return "", false
	}
	switch result.ActivationValue {
	case l.primary:
		return l.primaryKey, true
	case l.secondary:
		if l.secondaryKey != "dismiss" {
			return l.secondaryKey, true
		}
	case l.tertiary:
		if l.tertiaryKey != "dismiss" {
			return l.tertiaryKey, true
		}
	}
	return "", false
}

// appleScriptSafeText strips any character outside a conservative allowlist
// (letters, digits, spaces, and basic punctuation) before a string is
// interpolated into an AppleScript source string for the `display dialog`
// fallback below. strconv.Quote's Go-syntax escaping is not guaranteed to
// match AppleScript's own string-literal escaping rules (e.g. how it treats
// backslashes or embedded quotes), so this sanitizes first rather than
// relying on strconv.Quote alone to make the interpolation safe. Applied to
// every value that ends up inside the script string, including
// action.RepoName and any workspace/folder name folded into body/title.
func appleScriptSafeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '.', r == ',', r == '(', r == ')', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// showWorkspaceActionViaDialog is the original AppleScript `display dialog`
// implementation — centered, modal, no position control. Kept as the
// automatic fallback for machines without terminal-notifier installed.
func showWorkspaceActionViaDialog(ctx context.Context, action WorkspaceAction) error {
	title, body, primary, secondary, tertiary := workspaceActionLabels(action.Kind)
	primaryKey, secondaryKey, tertiaryKey := workspaceActionKeys(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}
	title = appleScriptSafeText(title)
	body = appleScriptSafeText(body)
	primary = appleScriptSafeText(primary)
	secondary = appleScriptSafeText(secondary)
	tertiary = appleScriptSafeText(tertiary)

	var script string
	if secondary == "" {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(primary), strconv.Quote(primary))
	} else if tertiary == "" {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s, %s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(secondary), strconv.Quote(primary), strconv.Quote(primary))
	} else {
		script = fmt.Sprintf(`display dialog %s with title %s buttons {%s, %s, %s} default button %s`,
			strconv.Quote(body), strconv.Quote(title), strconv.Quote(tertiary), strconv.Quote(secondary), strconv.Quote(primary), strconv.Quote(primary))
	}
	// The dialog is raised by a background process, so it has to be asked for
	// explicitly or it never reaches the front. Giving up keeps a forgotten
	// dialog from holding the process open for the rest of the session.
	script = `tell application "System Events" to activate` + "\n" + script + ` with icon note giving up after 120`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out))+strings.ToLower(err.Error()), "user canceled") {
			return nil
		}
		// Reporting the failure is what makes a broken dialog diagnosable
		// from the sync log instead of looking like nothing happened.
		return fmt.Errorf("show workspace dialog: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.Contains(string(out), "button returned:"+primary) {
		return ActivateWorkspaceAction(ctx, primaryKey, action.Workspace, action.Target)
	}
	if strings.Contains(string(out), "button returned:"+secondary) && secondaryKey != "dismiss" {
		return ActivateWorkspaceAction(ctx, secondaryKey, action.Workspace, action.Target)
	}
	if strings.Contains(string(out), "button returned:"+tertiary) && tertiaryKey != "dismiss" {
		return ActivateWorkspaceAction(ctx, tertiaryKey, action.Workspace, action.Target)
	}
	return nil
}
