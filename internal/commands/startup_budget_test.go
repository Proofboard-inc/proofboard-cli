package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

// The startup checks used to share one deadline, spent in order, so whatever
// the version check took was taken from the dictionary check after it. The
// dictionary reported "context deadline exceeded" on every single command
// while being reachable and 34 KB in size. Each network check must carry its
// own deadline derived from the caller's context, not from a budget another
// call has already spent.
func TestStartupChecksDoNotShareOneDeadline(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	body := string(src)

	for _, want := range []string{
		"context.WithTimeout(ctx, versionCheckBudget)",
		"context.WithTimeout(ctx, dictionaryBudget)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s: a network check is still sharing another call's deadline", want)
		}
	}

	// Derived from ctx, never from the short local budget, or the sharing is
	// reintroduced under a different name.
	if regexp.MustCompile(`context\.WithTimeout\(checkCtx,`).MatchString(body) {
		t.Error("a check derives its deadline from checkCtx, so it inherits time already spent")
	}
}

// Both network checks run on every command, including the sync a git hook
// fires on every commit, so both are throttled. An unthrottled check is a
// network round trip the developer pays for on each invocation.
func TestBothNetworkChecksAreThrottled(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	body := string(src)
	for _, field := range []string{"LastVersionCheck", "LastDictionaryUpdateCheck"} {
		if !strings.Contains(body, field) {
			t.Errorf("%s is not consulted, so that check runs on every command", field)
		}
	}
}

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

// The version and dictionary checks both begin by loading state, and state
// loading refuses an expired context. It used to run on the shared local-work
// budget, after shell-hook maintenance had spent it, so on the windows-latest
// runner both checks were skipped and printed nothing:
// TestStartupUpdateChecks got "" in 0.56s. On a slow Windows machine that is a
// developer who is never told an update exists and whose dictionary never
// refreshes.
func TestUpdateChecksSurviveAnExhaustedLocalWorkBudget(t *testing.T) {
	tempHome := t.TempDir()
	setTestHome(t, tempHome)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	ctx := context.Background()
	st := state.Default()
	st.AutoUpdateDictionary = false
	if err := state.NewStore(tempHome).Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"9.9.9","url":"https://proofboard.io/9.9.9"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_RELEASE_LATEST_VERSION_PATH", "/latest.json")
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)

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
	if !strings.Contains(out.String(), "A new version of Proofboard Career Agent is available") {
		t.Fatalf("update notice missing once local startup work ran out of time; got: %q", out.String())
	}
	saved, err := state.NewStore(tempHome).Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if saved.LastVersionCheck.IsZero() {
		t.Error("version check was not recorded, so it would be retried on every command")
	}
}
