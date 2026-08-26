package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/api"
)

// Login polls until the authorization is approved. The only thing that ever
// stopped it was a deadline derived from the server's expiresIn, so a response
// that omitted it — or sent zero — left the loop polling forever: no deadline,
// no cancellation, nothing to end it but killing the process.
//
// That is not hypothetical. It is what hung CI on macOS until the ten-minute
// per-package timeout fired, in internal/auth and internal/commands both, and
// on a real machine it is a CLI that prints "Waiting for authentication..."
// and never returns.
//
// The context here is deliberately Background: a test that supplies its own
// deadline would pass whether or not Login has a bound of its own, which is
// the entire question.
func TestLoginStopsWaitingWhenTheServerSendsNoExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/device-code"):
			// Deliberately no expiresIn: this is the case that never ended.
			_ = json.NewEncoder(w).Encode(api.DeviceCodeResponse{
				DeviceCode: "device-code",
				UserCode:   "ABCD-1234",
			})
		case strings.Contains(r.URL.Path, "/auth/poll/"):
			_ = json.NewEncoder(w).Encode(api.PollDeviceCodeResponse{Status: "pending"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("NO_BROWSER", "1")

	original := defaultAuthorizationWindow
	defaultAuthorizationWindow = 2 * time.Second
	t.Cleanup(func() { defaultAuthorizationWindow = original })

	client := api.NewClient(server.URL, "", "", "", "", "")
	service := NewService(NewCredentialStore(t.TempDir()), client)

	done := make(chan error, 1)
	go func() {
		_, err := service.Login(context.Background(), "email-hash")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected Login to fail once the authorization window closed")
		}
		if !strings.Contains(err.Error(), "authentication window closed") {
			t.Fatalf("Login() error = %v, want it to report the window closing", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Login never gave up: with no expiresIn from the server the " +
			"poll loop has no deadline of its own and waits forever")
	}
}
