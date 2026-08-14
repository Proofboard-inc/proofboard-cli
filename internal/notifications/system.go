package notifications

import (
	"fmt"
	"io"
	"strings"

	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/style"
)

type Event struct {
	Title           string
	Body            string
	PrimaryAction   string
	SecondaryAction string
	TertiaryAction  string
}

func PrintEvent(out io.Writer, event Event) {
	if out != nil {
		_, _ = fmt.Fprint(out, renderTerminal(out, event))
	}
}

// renderTerminal is the single rendering point for every notification the
// CLI prints (session expiry, sync/milestone confirmations, remote
// notifications surfaced by `proofboard notices`, etc.), so they all share
// one consistent branded look (matching the "New repository detected"
// prompt from `detect`): a green check, the bold-cyan "Proofboard" wordmark,
// a bold headline, and muted body text. Falls back to plain, uncolored text
// automatically for non-terminal output (style.Enabled handles that).
func renderTerminal(w io.Writer, event Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s\n",
		style.Success(w, "✓"),
		style.Brand(w, "Proofboard"),
		style.Heading(w, "— "+event.Title))
	if event.Body != "" {
		for _, line := range strings.Split(event.Body, "\n") {
			fmt.Fprintf(&b, "  %s\n", style.Muted(w, line))
		}
	}
	// Deliberately no "Actions: X | Y | Z" line here: on this plain-text
	// path (surfaced notifications, sync summaries) those aren't clickable,
	// so printing them read as broken UI. Real actions are only offered
	// through the actual interactive dialog/toast paths in workspace*.go,
	// which have genuine buttons.
	return b.String()
}


func RemoteNotification(n model.Notification) Event {
	meta := n.Meta
	get := func(keys ...string) string {
		for _, key := range keys {
			if v, ok := meta[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return ""
	}

	// Prefer the backend's own title/message when it sent one: that's the
	// real, specific copy (e.g. "4 new milestone clusters captured. Review
	// on your dashboard.") rather than reconstructing a generic canned
	// sentence from just the notification type. Most terminals auto-linkify
	// a bare URL, so the action URL is printed as plain text rather than a
	// fake "Actions:" button.
	if strings.TrimSpace(n.Title) != "" {
		body := n.Message
		if n.ActionURL != "" {
			if body != "" {
				body += "\n"
			}
			body += n.ActionURL
		}
		return Event{Title: n.Title, Body: body}
	}

	switch n.Type {
	case "milestone_bundle_ready":
		return MilestoneDetected(get("title", "milestoneTitle", "name"))
	case "proofboard_viewed":
		return Event{
			Title: "Someone viewed your Proofboard",
			Body:  get("message", "body", "description"),
		}
	case "vcs_sync_completed", "cli_sync_complete", "proof_asset_confirmed", "project_verified":
		milestones := 1
		if v, ok := meta["milestones"].(float64); ok && v > 0 {
			milestones = int(v)
		}
		return ProofOfShipCaptured(milestones)
	default:
		body := get("message", "body", "description")
		if body == "" {
			body = fmt.Sprintf("Open Proofboard to review this %s notification.", strings.ReplaceAll(n.Type, "_", " "))
		}
		return Event{
			Title:         notificationTypeTitle(n.Type),
			Body:          body,
			PrimaryAction: "Open Proofboard",
		}
	}
}

func notificationTypeTitle(notificationType string) string {
	title := strings.ReplaceAll(notificationType, "_", " ")
	if title == "" {
		return "Proofboard notification"
	}
	for index, character := range title {
		return strings.ToUpper(string(character)) + title[index+len(string(character)):]
	}
	return "Proofboard notification"
}

func NewProjectDetected(_ string) Event {
	return Event{
		Title:           "Project detected",
		Body:            "Would you like to add this project to your career record?",
		PrimaryAction:   "Sync Project",
		SecondaryAction: "Not Now / Never Ask Again",
	}
}

func ProjectSyncNeeded(projectName string) Event {
	return Event{
		Title:           "Project needs sync",
		Body:            fmt.Sprintf("%s\nThe Career Agent will capture the latest work automatically.", projectName),
		PrimaryAction:   "Sync Project",
		SecondaryAction: "Not Now",
	}
}

func ProofOfShipCaptured(milestoneCount int) Event {
	return Event{
		Title:           "Milestone detected",
		Body:            fmt.Sprintf("%d milestone(s) captured locally. Review and approve on your dashboard, then toggle the project public to publish your SHA proof.", milestoneCount),
		PrimaryAction:   "Review",
		SecondaryAction: "Publish",
		TertiaryAction:  "Skip",
	}
}

func MilestoneDetected(title string) Event {
	if strings.TrimSpace(title) == "" {
		title = "Engineering milestone"
	}
	return Event{
		Title:           "Milestone detected",
		Body:            title,
		PrimaryAction:   "Review",
		SecondaryAction: "Publish",
		TertiaryAction:  "Skip",
	}
}

func UpdateAvailable(version string) Event {
	return Event{
		Title:           "Update available",
		Body:            fmt.Sprintf("Proofboard Career Agent %s is available.", version),
		PrimaryAction:   "Update Now",
		SecondaryAction: "Later",
	}
}

func AuthExpiringSoon(days int) Event {
	return Event{
		Title:           "Your Proofboard session has expired",
		Body:            fmt.Sprintf("Your current session expires in %d days. The Career Agent will refresh it automatically when possible.", days),
		PrimaryAction:   "Reconnect",
		SecondaryAction: "Later",
	}
}

func SessionExpired() Event {
	return Event{
		Title:         "Your Proofboard session has expired",
		Body:          "Reconnect to resume private background synchronization.",
		PrimaryAction: "Reconnect",
	}
}
