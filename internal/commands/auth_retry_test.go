package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
)

func TestRetryAfterAuthUsesRefreshTokenSilently(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", "https://proofboard.io/agent/cli-auth")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/refresh" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":        retryTestJWT(time.Now().Add(time.Hour)),
			"refreshToken": "rotated-refresh",
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_API_REFRESH_PATH", "/refresh")

	store := pbauth.NewCredentialStore(homeDir)
	if err := store.Save(context.Background(), model.Credentials{
		Token:        "expired-access",
		RefreshToken: "refresh-token",
		EmailHash:    "email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var calls int
	var out bytes.Buffer
	err := retryAfterAuth(context.Background(), &out, "project sync", func() error {
		calls++
		if calls == 1 {
			return errors.New("API returned 401 Unauthorized")
		}
		credentials, loadErr := store.Load(context.Background())
		if loadErr != nil {
			return loadErr
		}
		if credentials.RefreshToken != "rotated-refresh" {
			return errors.New("refresh token was not rotated")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryAfterAuth() error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("operation calls = %d, want 2", calls)
	}
	if out.Len() != 0 {
		t.Fatalf("silent refresh wrote user-facing output: %q", out.String())
	}
}

func TestAutomaticAuthFailureDoesNotExposeAuthCommandUsage(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"device flow unavailable"}`, http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", server.URL+"/agent/cli-auth")

	var out bytes.Buffer
	err := runAuthFlow(context.Background(), &out, false)
	if err == nil {
		t.Fatal("expected automatic authentication failure")
	}
	if strings.Contains(out.String(), "Usage:") || strings.Contains(out.String(), "--rotate-key") {
		t.Fatalf("automatic flow exposed advanced auth usage: %q", out.String())
	}
	if strings.Contains(err.Error(), "proofboard auth") {
		t.Fatalf("automatic flow exposed auth command: %v", err)
	}
}

func TestCredentialsCompletelyExpiredRequiresMissingRefreshToken(t *testing.T) {
	expired := model.Credentials{Token: retryTestJWT(time.Now().Add(-time.Minute))}
	if !credentialsCompletelyExpired(expired) {
		t.Fatal("expected expired access-only credentials to require reconnect")
	}
	expired.RefreshToken = "refresh-token"
	if credentialsCompletelyExpired(expired) {
		t.Fatal("refreshable credentials must not require interactive reconnect")
	}
}

func TestDeferExpiredAgentSessionSuppressesDuplicatePromptDuringCooldown(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{Token: retryTestJWT(time.Now().Add(-time.Minute))}); err != nil {
		t.Fatalf("save expired credentials: %v", err)
	}
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	var out bytes.Buffer
	deferred, err := deferExpiredAgentSession(ctx, runtime, &out)
	if err != nil || !deferred {
		t.Fatalf("first defer = %v, %v", deferred, err)
	}
	if !strings.Contains(out.String(), "Your Proofboard session has expired") || !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("missing reconnect prompt: %q", out.String())
	}
	out.Reset()
	deferred, err = deferExpiredAgentSession(ctx, runtime, &out)
	if err != nil || !deferred || out.Len() != 0 {
		t.Fatalf("second defer = %v, %v, output=%q", deferred, err, out.String())
	}
}

func TestReconnectPromptRepeatsAfterCooldown(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	ctx := context.Background()
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	start := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer

	if err := promptAgentReconnectAt(ctx, &out, runtime, start); err != nil {
		t.Fatalf("first reconnect prompt: %v", err)
	}
	if !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("first reconnect prompt missing: %q", out.String())
	}

	out.Reset()
	if err := promptAgentReconnectAt(ctx, &out, runtime, start.Add(agentReconnectPromptInterval-time.Second)); err != nil {
		t.Fatalf("cooldown reconnect check: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("reconnect prompt repeated during cooldown: %q", out.String())
	}

	if err := promptAgentReconnectAt(ctx, &out, runtime, start.Add(agentReconnectPromptInterval)); err != nil {
		t.Fatalf("reconnect prompt after cooldown: %v", err)
	}
	if !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("reconnect prompt did not repeat after cooldown: %q", out.String())
	}
}

func TestLegacyReconnectStateWithoutTimestampPromptsAgain(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	ctx := context.Background()
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	current, err := runtime.state.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	current.AuthReconnectPrompted = true
	if err := runtime.state.Save(ctx, current); err != nil {
		t.Fatalf("save legacy state: %v", err)
	}

	var out bytes.Buffer
	if err := promptAgentReconnectAt(ctx, &out, runtime, time.Now()); err != nil {
		t.Fatalf("legacy reconnect prompt: %v", err)
	}
	if !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("legacy reconnect state remained permanently suppressed: %q", out.String())
	}
}

func TestDeferExpiredAgentSessionPromptsWhenRefreshIsRejected(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "expired", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_API_REFRESH_PATH", "/refresh")
	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{
		Token:        retryTestJWT(time.Now().Add(-time.Minute)),
		RefreshToken: "expired-refresh-token",
	}); err != nil {
		t.Fatalf("save expired credentials: %v", err)
	}
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	var out bytes.Buffer
	deferred, err := deferExpiredAgentSession(ctx, runtime, &out)
	if err != nil || !deferred {
		t.Fatalf("defer = %v, %v", deferred, err)
	}
	if !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("missing reconnect prompt: %q", out.String())
	}
}

func TestDeferExpiredAgentSessionDoesNotReconnectForTransientRefreshFailure(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	t.Setenv("PROOFBOARD_API_REFRESH_PATH", "/refresh")
	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{
		Token:        retryTestJWT(time.Now().Add(-time.Minute)),
		RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	var out bytes.Buffer
	deferred, err := deferExpiredAgentSession(ctx, runtime, &out)
	if err == nil || deferred {
		t.Fatalf("defer = %v, %v; want transient error", deferred, err)
	}
	if out.Len() != 0 {
		t.Fatalf("transient failure prompted reconnect: %q", out.String())
	}
}

func TestRetryAfterAuthForAgentDefersBrowserUntilReconnect(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")
	ctx := context.Background()
	if err := pbauth.NewCredentialStore(homeDir).Save(ctx, model.Credentials{
		Token: retryTestJWT(time.Now().Add(time.Hour)),
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	runtime, err := loadRuntime(ctx)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	var out bytes.Buffer
	err = retryAfterAuthForAgent(ctx, &out, runtime, func() error {
		return errors.New("API returned 401 Unauthorized")
	})
	if !errors.Is(err, errAgentReconnectRequired) {
		t.Fatalf("retry error = %v, want reconnect sentinel", err)
	}
	if !strings.Contains(out.String(), "Reconnect") {
		t.Fatalf("missing reconnect prompt: %q", out.String())
	}
}

func retryTestJWT(expiry time.Time) string {
	header, _ := json.Marshal(map[string]string{"alg": "none"})
	payload, _ := json.Marshal(map[string]int64{"exp": expiry.Unix()})
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
