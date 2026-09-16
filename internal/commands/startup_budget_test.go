package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
)

// The expired-session notice was the last thing done under one shared
// 400ms budget for all the local work that precedes every command: loading
// runtime, maintaining shell hooks, reading and writing state, loading the
// dictionary. notifyAuthExpiry's first step, loading credentials, fails
// immediately on an expired context, and it treats any error as nothing to
// report — so wherever that local work ran slow, the notice silently vanished.
//
// It did on CI: this whole check takes about 0.01s on Linux and 2.04s on the
// windows-latest runner, where TestStartupUpdateChecksSurfacesDesktopNotifications
// failed with an empty output. On a slow Windows laptop it means a developer is
// never told their session expired.
func TestExpiredSessionNoticeSurvivesAnExhaustedLocalWorkBudget(t *testing.T) {
	tempHome := t.TempDir()
	setTestHome(t, tempHome)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	ctx := context.Background()
	if err := pbauth.NewCredentialStore(tempHome).Save(ctx, model.Credentials{
		Token:     testJWT(time.Now().Add(-time.Minute).UTC()),
		EmailHash: "email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	st := state.Default()
	st.AutoUpdateDictionary = false
	if err := state.NewStore(tempHome).Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)

	// Stand in for a machine slow enough that the local work uses up its
	// budget before the notice is reached.
	original := startupLocalWorkBudget
	startupLocalWorkBudget = time.Nanosecond
	t.Cleanup(func() { startupLocalWorkBudget = original })

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "status"}
	parent := &cobra.Command{Use: "proofboard"}
	parent.AddCommand(cmd)
	cmd.SetOut(&out)

	if err := runStartupUpdateChecks(ctx, cmd); err != nil {
		t.Fatalf("runStartupUpdateChecks: %v", err)
	}
	if !strings.Contains(out.String(), "Your Proofboard session has expired") {
		t.Fatalf("expired-session notice missing once local startup work ran out of time; got: %q", out.String())
	}
}
