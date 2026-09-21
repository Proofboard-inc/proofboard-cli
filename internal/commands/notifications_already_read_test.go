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

// The CLI asks for isRead=false, but the backend's validation pipe converts the
// string "false" with Boolean(), which is true, so it answered with notifications
// already read. Each was printed and "marked read" again, and the same
// "Open-source projects found" notice appeared on every single command,
// including the sync a git hook runs on each push. A notification the server
// itself reports as read has been handled; printing it again is noise.
func TestStartupSkipsNotificationsTheServerReportsAsRead(t *testing.T) {
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
	st := state.Default()
	st.AutoUpdateDictionary = false
	st.LastVersionCheck = time.Now().UTC()
	if err := state.NewStore(tempHome).Save(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var mu sync.Mutex
	var patches []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			mu.Lock()
			patches = append(patches, r.URL.Path)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[` +
			`{"id":"old","type":"oss_projects_discovered","title":"Open-source projects found","message":"We found 1 open-source project","isRead":true},` +
			`{"id":"new","type":"sync_completed","title":"Sync completed","message":"m","isRead":false}` +
			`],"meta":{"total":2,"page":1,"limit":20}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PROOFBOARD_RELEASE_BASE_URL", srv.URL)
	t.Setenv("PROOFBOARD_API_BASE_URL", srv.URL)

	parent := &cobra.Command{Use: "proofboard"}
	cmd := &cobra.Command{Use: "status"}
	parent.AddCommand(cmd)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runStartupUpdateChecks(ctx, cmd); err != nil {
		t.Fatalf("runStartupUpdateChecks: %v", err)
	}

	if bytes.Contains(out.Bytes(), []byte("Open-source projects found")) {
		t.Errorf("printed a notification the server reports as already read: %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Sync completed")) {
		t.Errorf("unread notification missing from output: %q", out.String())
	}
	mu.Lock()
	defer mu.Unlock()
	for _, p := range patches {
		if p == "/api/v1/notifications/old/read" || p == "/api/v1/cli/notifications/old/read" {
			t.Errorf("re-marked an already-read notification: %v", patches)
		}
	}
}
