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
	"strconv"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/config"
	"github.com/proofboard/proofboard/internal/crypto"
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
	// The notifications route belongs to the web application API and does not
	// accept CLI-scoped JWTs. Avoid a guaranteed 401 (and a misleading auth
	// log entry) until the backend exposes a CLI notifications endpoint.
	if scope, scopeErr := crypto.JWTScope(credentials.Token); scopeErr == nil && scope == "cli" {
		return
	}
	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "20")
	query.Set("isRead", "false")
	res, err := runtime.api.GetNotifications(ctx, credentials.Token, query)
	if err != nil {
		return
	}
	for _, n := range res.Data {
		if n.Type == "milestone_bundle_ready" {
			bundleID := notificationMetaString(n.Meta, "bundleId", "milestoneBundleId", "id")
			title := notificationMetaString(n.Meta, "title", "milestoneTitle", "name")
			commitCount, hasCommitCount := notificationMetaInt(n.Meta, "commitCount", "commitsCount", "count")
			// Milestones are rare enough, and reviewable purely from the
			// terminal (review/publish/skip are all real commands below), so
			// this no longer also raises an OS-level popup the way it used
			// to — the terminal print, with the next commands spelled out
			// literally, is the only surface now.
			printMilestoneReady(out, title, commitCount, hasCommitCount, bundleID)
		} else {
			// Terminal-only: these are routine, already-happened confirmations
			// (sync completed, someone viewed your proofboard, etc.) surfaced
			// the next time the user happens to run a command — not the kind
			// of time-sensitive prompt that warrants interrupting them with an
			// OS-level popup on top of it.
			notifications.PrintEvent(out, notifications.RemoteNotification(n))
		}
		_ = runtime.api.MarkNotificationRead(ctx, credentials.Token, n.ID)
	}
}

// printMilestoneReady prints the milestone-ready block that used to be an
// OS-level notification with Review/Publish/Skip buttons. With no buttons to
// click anymore, the three actions are spelled out as literal, runnable
// `proofboard milestone ...` commands instead.
func printMilestoneReady(out io.Writer, title string, commitCount int, hasCommitCount bool, bundleID string) {
	if strings.TrimSpace(title) == "" {
		title = "Engineering milestone"
	}
	countSuffix := ""
	if hasCommitCount {
		unit := "commit"
		if commitCount != 1 {
			unit = "commits"
		}
		countSuffix = fmt.Sprintf(" (%d %s)", commitCount, unit)
	}
	fmt.Fprintf(out, "%s Milestone ready: %q%s\n", style.Success(out, "✓"), title, countSuffix)
	if strings.TrimSpace(bundleID) == "" {
		// Nothing to act on without a bundle id — the caller has nothing
		// further to run, so leave it at the headline.
		return
	}
	type suggestedCommand struct {
		command string
		help    string
	}
	commands := []suggestedCommand{
		{fmt.Sprintf("Run `proofboard milestone review %s`", bundleID), "inspect before publishing"},
		{fmt.Sprintf("Run `proofboard milestone publish %s`", bundleID), "publish as-is"},
		{fmt.Sprintf("Run `proofboard milestone skip %s`", bundleID), "dismiss this one"},
	}
	width := 0
	for _, c := range commands {
		if len(c.command) > width {
			width = len(c.command)
		}
	}
	for _, c := range commands {
		fmt.Fprintf(out, "  %-*s — %s\n", width, c.command, c.help)
	}
}

// notificationMetaInt is notificationMetaString's numeric counterpart: JSON
// metadata decodes numbers as float64, but a backend that serializes it as a
// string is tolerated too.
func notificationMetaInt(meta map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := meta[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func notificationMetaString(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := meta[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

