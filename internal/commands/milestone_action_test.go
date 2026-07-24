package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
)

func TestMilestoneActionPublishesBundle(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/milestone-bundles/bundle-1/approve" {
			called = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("PROOFBOARD_API_BASE_URL", server.URL)
	if err := pbauth.NewCredentialStore(homeDir).Save(context.Background(), model.Credentials{Token: "token"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	var out bytes.Buffer
	cmd := newMilestoneActionCommand(context.Background(), &out)
	cmd.SetArgs([]string{"publish", "bundle-1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("milestone publish: %v", err)
	}
	if !called {
		t.Fatal("expected milestone approval endpoint to be called")
	}
}
