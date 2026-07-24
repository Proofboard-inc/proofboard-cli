package notifications

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gen2brain/beeep"
	"github.com/proofboard/proofboard/internal/model"
)

type Event struct {
	Title           string
	Body            string
	PrimaryAction   string
	SecondaryAction string
	TertiaryAction  string
}

func Dispatch(out io.Writer, event Event) {
	PrintEvent(out, event)
	_ = NotifyDesktop(event)
}

func PrintEvent(out io.Writer, event Event) {
	if out != nil {
		_, _ = fmt.Fprintln(out, renderTerminal(event))
	}
}

func renderTerminal(event Event) string {
	var b strings.Builder
	b.WriteString("Proofboard: ")
	b.WriteString(event.Title)
	if event.Body != "" {
		b.WriteString("\n")
		b.WriteString(event.Body)
	}
	if event.PrimaryAction != "" || event.SecondaryAction != "" || event.TertiaryAction != "" {
		b.WriteString("\n")
		actions := make([]string, 0, 3)
		for _, action := range []string{event.PrimaryAction, event.SecondaryAction, event.TertiaryAction} {
			if action != "" {
				actions = append(actions, action)
			}
		}
		b.WriteString("Actions: ")
		b.WriteString(strings.Join(actions, " | "))
	}
	return b.String()
}

func NotifyDesktop(event Event) error {
	if os.Getenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS") == "1" {
		return nil
	}
	return beeep.Notify(event.Title, event.Body, "")
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

	switch n.Type {
	case "milestone_bundle_ready":
		return MilestoneDetected(get("title", "milestoneTitle", "name"))
	case "proposal_viewed", "proposal_accepted", "proposal_declined", "proofboard_viewed":
		role := get("role", "roleTitle", "title")
		org := get("company", "companyName", "organization", "org")
		if role == "" {
			role = "Inbound opportunity"
		}
		if org == "" {
			org = "Proofboard"
		}
		reasons := []string{}
		if v := get("reason", "message", "body"); v != "" {
			reasons = append(reasons, v)
		}
		return InboundOpportunity(role, org, reasons)
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
			Title:         strings.Title(strings.ReplaceAll(n.Type, "_", " ")),
			Body:          body,
			PrimaryAction: "Open Proofboard",
		}
	}
}

func NewProjectDetected(projectName string) Event {
	return Event{
		Title:           "New repository detected",
		Body:            fmt.Sprintf("%s\nWould you like Proofboard to track this project? Processing stays local and no proprietary source code leaves your computer.", projectName),
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

func InboundOpportunity(role, org string, reasons []string) Event {
	return Event{
		Title:           "New opportunity match",
		Body:            strings.Join(append([]string{fmt.Sprintf("%s at %s", role, org), "Why you matched:"}, reasons...), "\n"),
		PrimaryAction:   "View Dealboard",
		SecondaryAction: "Dismiss",
	}
}

func ProofOfShipCaptured(milestoneCount int) Event {
	return Event{
		Title:           "Milestone detected",
		Body:            fmt.Sprintf("%d milestone(s) were captured locally and added to your engineering proof.", milestoneCount),
		PrimaryAction:   "Review",
		SecondaryAction: "Publish",
		TertiaryAction:  "Ignore",
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
		TertiaryAction:  "Ignore",
	}
}

func MonthlyCareerSummary(monthName string) Event {
	return Event{
		Title:           fmt.Sprintf("%s summary is ready", monthName),
		Body:            fmt.Sprintf("Your career summary for %s is now available.", monthName),
		PrimaryAction:   "View Summary",
		SecondaryAction: "Later",
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
