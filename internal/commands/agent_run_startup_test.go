package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

// The background agent is started detached, with its output going to
// /dev/null or a service journal. It used to run the same startup checks as an
// interactive command: fetch unread notifications, print them, and mark each one
// read on the server. Nobody reads that output, so every agent start consumed
// the developer's notifications unseen. The checks also ran before the agent
// claimed its pid, so `proofboard update` could give up waiting for the agent
// and report "process did not become active" while it was starting normally.
func TestDetachedAgentRunConsumesNoNotificationsAtStartup(t *testing.T) {
	tempHome := t.TempDir()
	setTestHome(t, tempHome)
	t.Setenv("PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS", "1")

	ctx := context.Background()
	if err := pbauth.NewCredentialStore(tempHome).Save(ctx, model.Credentials{
		Token:     testJWT(time.Now().Add(time.Hour).UTC()),
		EmailHash: "email-hash",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	if err := state.NewStore(tempHome).Save(ctx, state.Default()); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var mu sync.Mutex
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"n1","type":"sync_completed","title":"Sync completed","message":"m"}],"total":1,"page":1,"limit":20}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)

	root := &cobra.Command{Use: "proofboard"}
	agent := &cobra.Command{Use: "agent"}
	run := &cobra.Command{Use: "run"}
	root.AddCommand(agent)
	agent.AddCommand(run)
	var out bytes.Buffer
	run.SetOut(&out)

	if err := runStartupUpdateChecks(ctx, run); err != nil {
		t.Fatalf("runStartupUpdateChecks() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 0 {
		t.Errorf("detached agent run made startup requests %v; notifications are marked read with nobody to see them", requests)
	}
	if out.Len() != 0 {
		t.Errorf("detached agent run printed startup output nobody reads: %q", out.String())
	}
}
