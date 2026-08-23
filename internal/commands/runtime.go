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
	for _, n := range res.Data {
		switch n.Type {
		case "milestone_bundle_ready":
			// A sync can produce several milestone clusters at once, each
			// raising its own milestone_bundle_ready notification server
			// side, so printing one line per bundle here would be noisy. The
			// cli_sync_complete/vcs_sync_completed notification already
			// tells the developer their work was captured; these are marked
			// read below without printing anything.
		case "proposal_viewed", "proposal_accepted", "proposal_declined":
			// Proposals/Dealboard aren't part of the CLI experience right
			// now: marked read below without printing anything.
		default:
			// Terminal-only: these are routine, already-happened confirmations
			// (sync completed, someone viewed your proofboard, etc.) surfaced
			// the next time the user happens to run a command, not the kind
			// of time-sensitive prompt that warrants interrupting them with an
			// OS-level popup on top of it.
			notifications.PrintEvent(out, notifications.RemoteNotification(n))
		}
		if isCliScope {
			_ = runtime.api.MarkCliNotificationRead(ctx, credentials.Token, n.ID)
		} else {
			_ = runtime.api.MarkNotificationRead(ctx, credentials.Token, n.ID)
		}
	}
}
