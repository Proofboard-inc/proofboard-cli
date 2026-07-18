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
}

func Dispatch(out io.Writer, event Event) {
	if out != nil {
		_, _ = fmt.Fprintln(out, renderTerminal(event))
	}
	_ = NotifyDesktop(event)
}

func renderTerminal(event Event) string {
	var b strings.Builder
	b.WriteString("Proofboard: ")
	b.WriteString(event.Title)
	if event.Body != "" {
		b.WriteString("\n")
		b.WriteString(event.Body)
	}
	if event.PrimaryAction != "" || event.SecondaryAction != "" {
		b.WriteString("\n")
		if event.PrimaryAction != "" {
			b.WriteString("Primary: ")
			b.WriteString(event.PrimaryAction)
		}
		if event.PrimaryAction != "" && event.SecondaryAction != "" {
			b.WriteString(" | ")
		}
		if event.SecondaryAction != "" {
			b.WriteString("Secondary: ")
			b.WriteString(event.SecondaryAction)
		}
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
		Title:           "New project detected",
		Body:            fmt.Sprintf("%s\nAdd it to Proofboard to start tracking this repo.", projectName),
		PrimaryAction:   "proofboard link",
		SecondaryAction: "Not this project",
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
		Title:         "Proof-of-Ship captured",
		Body:          fmt.Sprintf("%d milestones captured.\nYour work has been added to your proofboard.", milestoneCount),
		PrimaryAction: "View Dashboard",
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
		Body:            fmt.Sprintf("Proofboard CLI %s is available.\nRun \"proofboard update\" to install.", version),
		PrimaryAction:   "Update Now",
		SecondaryAction: "Later",
	}
}

func AuthExpiringSoon(days int) Event {
	return Event{
		Title:           "Re-authentication needed",
		Body:            fmt.Sprintf("Your session expires in %d days.\nRun \"proofboard auth\" to continue syncing without interruption.", days),
		PrimaryAction:   "Re-authenticate",
		SecondaryAction: "Later",
	}
}
