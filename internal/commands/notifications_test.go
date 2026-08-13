package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func TestStartupUpdateChecksSurfacesDesktopNotifications(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	expiry := time.Now().Add(-time.Minute).UTC()
	token := testJWT(expiry)

	credStore := pbauth.NewCredentialStore(tempHome)
	if err := credStore.Save(context.Background(), model.Credentials{
		Token:     token,
		EmailHash: "email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = false
	if err := stateStore.Save(context.Background(), st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"version": %q, "url": "https://proofboard.io/%s"}`, version.Version, version.Version)))
		case "/api/v1/notifications":
			if got := r.URL.Query().Get("isRead"); got != "false" {
				t.Fatalf("expected isRead=false, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(model.PaginatedNotifications{
				Data: []model.Notification{
					{
						ID:     "notif-1",
						Type:   "proposal_accepted",
						IsRead: false,
						Meta: map[string]any{
							"roleTitle":   "Backend Engineer",
							"companyName": "Fintech Labs",
							"reason":      "14 authentication-related milestones",
						},
					},
				},
				Meta: model.PaginationMeta{Total: 1, Page: 1, Limit: 20, TotalPages: 1},
			})
		case "/api/v1/notifications/notif-1/read":
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH on notification read, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_DICTIONARY_PATH", "/dictionary/latest.json")

	ctx := context.Background()
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)
	cmd.SetOut(&out)

	if err := runStartupUpdateChecks(ctx, cmd); err != nil {
		t.Fatalf("runStartupUpdateChecks: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Your Proofboard session has expired") {
		t.Fatalf("expected expired-session notification, got: %q", output)
	}
	if !strings.Contains(output, "New opportunity match") {
		t.Fatalf("expected inbound opportunity notification, got: %q", output)
	}
}

// A CLI-scoped token is a completely separate auth strategy from the user
// session JWT (see backend CLAUDE.md's "Three independent JWT strategies"),
// so it must call the CLI-guarded mirror /api/v1/cli/notifications rather
// than the user-session-only /api/v1/notifications, which rejects it
// outright. This used to just skip surfacing notifications entirely for CLI
// tokens — silently disabling sync-complete/milestone-ready notices for
// every real (device-code-authenticated) CLI install.
func TestStartupUpdateChecksUsesCliScopedRouteForCLIToken(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"exp":   time.Now().Add(time.Hour).Unix(),
		"scope": "cli",
	})
	token := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	credStore := pbauth.NewCredentialStore(tempHome)
	if err := credStore.Save(context.Background(), model.Credentials{Token: token}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stateStore := state.NewStore(tempHome)
	st := state.Default()
	st.AutoUpdateDictionary = false
	if err := stateStore.Save(context.Background(), st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	webNotificationCalls := 0
	cliNotificationCalls := 0
	markReadCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"version": %q, "url": "https://proofboard.io/%s"}`, version.Version, version.Version)))
		case "/api/v1/notifications":
			webNotificationCalls++
			http.Error(w, "CLI tokens must not call this route", http.StatusUnauthorized)
		case "/api/v1/cli/notifications":
			cliNotificationCalls++
			if got := r.URL.Query().Get("isRead"); got != "false" {
				t.Fatalf("expected isRead=false, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(model.PaginatedNotifications{
				Data: []model.Notification{
					{ID: "notif-1", Type: "cli_sync_complete", IsRead: false, Meta: map[string]any{}},
				},
				Meta: model.PaginationMeta{Total: 1, Page: 1, Limit: 20, TotalPages: 1},
			})
		case "/api/v1/cli/notifications/notif-1/read":
			markReadCalls++
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH on cli notification read, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)
	cmd.SetOut(&out)
	if err := runStartupUpdateChecks(context.Background(), cmd); err != nil {
		t.Fatalf("runStartupUpdateChecks: %v", err)
	}
	if webNotificationCalls != 0 {
		t.Fatalf("web notification endpoint called %d times with a CLI-scoped token", webNotificationCalls)
	}
	if cliNotificationCalls != 1 {
		t.Fatalf("expected exactly one call to the CLI notifications endpoint, got %d", cliNotificationCalls)
	}
	if markReadCalls != 1 {
		t.Fatalf("expected the surfaced notification to be marked read via the CLI route, got %d calls", markReadCalls)
	}
	if !strings.Contains(out.String(), "Proofboard") {
		t.Fatalf("expected the CLI-scoped notification to be printed, got: %q", out.String())
	}
}

func TestNotifyAuthExpiryStaysSilentWhenRefreshTokenExists(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	store := pbauth.NewCredentialStore(homeDir)
	if err := store.Save(context.Background(), model.Credentials{
		Token:        testJWT(time.Now().Add(-time.Hour)),
		RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	runtime, err := loadRuntime(context.Background())
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	var out bytes.Buffer
	notifyAuthExpiry(context.Background(), &out, runtime)
	if out.Len() != 0 {
		t.Fatalf("expected refreshable session to remain silent, got %q", out.String())
	}
}

func testJWT(expiry time.Time) string {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"exp": expiry.Unix()})
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
