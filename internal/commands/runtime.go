package commands

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/config"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/notifications"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/style"
)

type runtimeContext struct {
	config      config.Config
	homeDir     string
	workingDir  string
	credentials pbauth.CredentialStore
	state       state.Store
	api         api.Client
}

func loadRuntime(ctx context.Context) (runtimeContext, error) {
	cfg, err := config.Load(ctx)
	if err != nil {
		return runtimeContext{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return runtimeContext{}, fmt.Errorf("detect home directory: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return runtimeContext{}, fmt.Errorf("detect working directory: %w", err)
	}
	return runtimeContext{
		config:      cfg,
		homeDir:     home,
		workingDir:  wd,
		credentials: pbauth.NewCredentialStore(home),
		state:       state.NewStore(home),
		api:         api.NewClient(cfg.APIBaseURL, cfg.LinkPath, cfg.CheckPath, cfg.SyncPath, cfg.DeviceKeyRegistrationPath, cfg.RefreshPath),
	}, nil
}

func logPath(home string) string {
	return filepath.Join(home, ".proofboard", "sync.log")
}

func authEmailHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", "user.email")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read git config user.email: %w", err)
	}
	email := strings.TrimSpace(string(out))
	if email == "" {
		return "", fmt.Errorf("git config user.email is empty")
	}
	return crypto.NormalizedSHA256(email), nil
}

func notifyAuthExpiry(ctx context.Context, out io.Writer, runtime runtimeContext) {
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil || credentials.Token == "" || credentials.RefreshToken != "" {
		return
	}
	expiry, err := crypto.JWTExpiry(credentials.Token)
	if err != nil {
		return
	}
	until := time.Until(expiry)
	if until > 0 {
		return
	}
	notifications.PrintEvent(out, notifications.SessionExpired())
	if os.Getenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS") != "1" {
		_ = launchActionNotification(ctx, "reconnect")
	}
}

func launchActionNotification(ctx context.Context, kind string) error {
	return launchTargetActionNotification(ctx, kind, "", "")
}

func launchTargetActionNotification(ctx context.Context, kind, target, title string) error {
	if os.Getenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS") == "1" || !desktopNotificationsAvailable() {
		return nil
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	args := []string{"notify", "--kind", kind}
	if target != "" {
		args = append(args, "--target", target)
	}
	if title != "" {
		args = append(args, "--repo-name", title)
	}
	cmd := exec.CommandContext(ctx, execPath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return startDetachedCommand(cmd)
}

func desktopNotificationsAvailable() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return strings.TrimSpace(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) != "" ||
		strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func surfaceUnreadNotifications(ctx context.Context, out io.Writer, runtime runtimeContext) {
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil || credentials.Token == "" {
		return
	}
	// The CLI's device-code-issued token is a completely separate auth
	// strategy from the user session JWT (three independent JWT strategies;
	// see backend CLAUDE.md), so it must call the CLI-guarded mirror
	// (/api/v1/cli/notifications) rather than the user-session-only
	// /api/v1/notifications route, which rejects it outright.
	isCliScope := false
	if scope, scopeErr := crypto.JWTScope(credentials.Token); scopeErr == nil && scope == "cli" {
		isCliScope = true
	}
	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "20")
	query.Set("isRead", "false")
	var res model.PaginatedNotifications
	if isCliScope {
		res, err = runtime.api.GetCliNotifications(ctx, credentials.Token, query)
	} else {
		res, err = runtime.api.GetNotifications(ctx, credentials.Token, query)
	}
	if err != nil {
		return
	}
	markRead := func(id string) {
		if isCliScope {
			_ = runtime.api.MarkCliNotificationRead(ctx, credentials.Token, id)
		} else {
			_ = runtime.api.MarkNotificationRead(ctx, credentials.Token, id)
		}
	}

	var milestones []struct{ title, bundleID string }

	for _, n := range res.Data {
		switch n.Type {
		case "milestone_bundle_ready":
			// Collected rather than printed one at a time: a single sync can
			// raise one of these per cluster, and a separate block for each
			// would bury the rest of the output. They are printed together
			// below, with their identifiers, because those identifiers are
			// the only thing that makes `proofboard milestone` usable — the
			// subcommands take one and nothing else in the CLI prints one.
			milestones = append(milestones, struct{ title, bundleID string }{
				title:    notificationMetaString(n.Meta, "title", "milestoneTitle", "name"),
				bundleID: notificationMetaString(n.Meta, "bundleId", "milestoneBundleId", "id"),
			})
			markRead(n.ID)
		case "proposal_viewed", "proposal_accepted", "proposal_declined":
			// Deliberately left unread. Proposals are not part of the CLI
			// experience, so nothing is printed for them — and marking them
			// read anyway would consume them here and clear them from the
			// dashboard, where they ARE surfaced, without anyone having seen
			// them. Not displaying something is not the same as handling it.
		default:
			// Routine, already-happened confirmations (sync completed, your
			// proofboard was viewed) surfaced the next time a command runs.
			notifications.PrintEvent(out, notifications.RemoteNotification(n))
			markRead(n.ID)
		}
	}

	printMilestonesReady(out, milestones)
}

// printMilestonesReady prints the milestones waiting for a decision, followed
// by the commands that act on them. The bundle identifier is included because
// `proofboard milestone review|publish|skip` each require one, and this is the
// only place the CLI can learn it — without this the three subcommands exist
// but cannot be invoked.
func printMilestonesReady(out io.Writer, milestones []struct{ title, bundleID string }) {
	if len(milestones) == 0 {
		return
	}
	noun := "milestone"
	if len(milestones) != 1 {
		noun = "milestones"
	}
	fmt.Fprintf(out, "%s %s %s\n",
		style.Success(out, "✓"),
		style.Brand(out, "Proofboard"),
		style.Heading(out, fmt.Sprintf("— %d %s ready to review", len(milestones), noun)))

	for _, m := range milestones {
		title := strings.TrimSpace(m.title)
		if title == "" {
			title = "Engineering milestone"
		}
		if strings.TrimSpace(m.bundleID) == "" {
			// Nothing actionable without an identifier, so the headline is
			// all there is to say about this one.
			fmt.Fprintf(out, "  %s\n", style.Muted(out, title))
			continue
		}
		fmt.Fprintf(out, "  %s\n", title)
		fmt.Fprintf(out, "    %s  %s\n", style.Accent(out, "proofboard milestone review "+m.bundleID), style.Muted(out, "inspect it first"))
		fmt.Fprintf(out, "    %s  %s\n", style.Accent(out, "proofboard milestone publish "+m.bundleID), style.Muted(out, "publish as-is"))
		fmt.Fprintf(out, "    %s  %s\n", style.Accent(out, "proofboard milestone skip "+m.bundleID), style.Muted(out, "dismiss it"))
	}
}

// notificationMetaString reads the first non-empty string among keys from a
// notification's metadata, tolerating the different spellings the backend has
// used for the same field.
func notificationMetaString(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := meta[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
