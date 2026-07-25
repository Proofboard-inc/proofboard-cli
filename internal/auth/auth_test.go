package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/model"
)

func TestAuthorizationURLPrefillsDeviceCode(t *testing.T) {
	service := NewService(CredentialStore{}, api.Client{}, "https://proofboard.io/agent/cli-auth?source=desktop")
	got := service.authorizationURL("ABCD-1234")
	want := "https://proofboard.io/agent/cli-auth?code=ABCD-1234&source=desktop"
	if got != want {
		t.Fatalf("authorizationURL() = %q, want %q", got, want)
	}
}

func TestResolveAuthorizationURLPrefersCareerAgentRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/cli-auth" || r.URL.Query().Get("code") != "ABCD-1234" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	service := NewService(CredentialStore{}, api.Client{}, server.URL+"/agent/cli-auth")
	got, err := service.resolveAuthorizationURL(context.Background(), "ABCD-1234", "https://fallback.example/cli-auth")
	if err != nil {
		t.Fatalf("resolveAuthorizationURL() error: %v", err)
	}
	want := server.URL + "/agent/cli-auth?code=ABCD-1234"
	if got != want {
		t.Fatalf("resolveAuthorizationURL() = %q, want %q", got, want)
	}
}

func TestResolveAuthorizationURLAlwaysReturnsTheConfiguredPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Authorize Proofboard Career Agent"))
	}))
	t.Cleanup(server.Close)
	service := NewService(CredentialStore{}, api.Client{}, server.URL+"/cli-auth")

	got, err := service.resolveAuthorizationURL(context.Background(), "ABCD-1234", "")
	if err != nil {
		t.Fatalf("resolveAuthorizationURL() error: %v", err)
	}
	if got != server.URL+"/cli-auth?code=ABCD-1234" {
		t.Fatalf("resolveAuthorizationURL() = %q", got)
	}
}

func TestResolveAuthorizationURLDoesNotBlockOnAnUnreachablePage(t *testing.T) {
	// A page that cannot be reached, or one that carries not-found text in its
	// payload, must still be handed over: refusing to open it is what stopped
	// engineers from connecting at all.
	unreachable := httptest.NewServer(http.NotFoundHandler())
	unreachable.Close()
	service := NewService(CredentialStore{}, api.Client{}, unreachable.URL+"/cli-auth")

	got, err := service.resolveAuthorizationURL(context.Background(), "ABCD-1234", "")
	if err != nil {
		t.Fatalf("an unreachable page must not fail the connection: %v", err)
	}
	if got != unreachable.URL+"/cli-auth?code=ABCD-1234" {
		t.Fatalf("resolveAuthorizationURL() = %q", got)
	}
}

func TestRefreshRotatesAndPersistsCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/refresh" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var request api.RefreshTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if request.RefreshToken != "old-refresh" {
			t.Fatalf("refresh token = %q", request.RefreshToken)
		}
		_ = json.NewEncoder(w).Encode(api.RefreshTokenResponse{
			AccessToken:  testJWT(time.Now().Add(time.Hour)),
			RefreshToken: "new-refresh",
		})
	}))
	t.Cleanup(server.Close)

	store := NewCredentialStore(t.TempDir())
	client := api.NewClient(server.URL, "", "", "", "", "/refresh")
	service := NewService(store, client)
	refreshed, err := service.Refresh(context.Background(), model.Credentials{
		Token:        "old-access",
		RefreshToken: "old-refresh",
		EmailHash:    "email-hash",
	})
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if refreshed.Token == "" || refreshed.RefreshToken != "new-refresh" || refreshed.EmailHash != "email-hash" {
		t.Fatalf("unexpected refreshed credentials: %+v", refreshed)
	}
	persisted, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load persisted credentials: %v", err)
	}
	if persisted.Token != refreshed.Token || persisted.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected persisted credentials: %+v", persisted)
	}
}

func testJWT(expiry time.Time) string {
	encode := func(value any) string {
		data, _ := json.Marshal(value)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	return encode(map[string]string{"alg": "none"}) + "." + encode(map[string]int64{"exp": expiry.Unix()}) + ".sig"
}
