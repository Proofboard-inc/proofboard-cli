package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMilestoneBundleAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/projects/milestone-bundles/bundle-1/approve":
			if r.Method != http.MethodPost {
				t.Fatalf("approve method = %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		case "/api/v1/projects/milestone-bundles/bundle-1/decline":
			if r.Method != http.MethodPost {
				t.Fatalf("decline method = %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "", "", "")

	if err := client.ApproveMilestoneBundle(context.Background(), "token", "bundle-1"); err != nil {
		t.Fatalf("ApproveMilestoneBundle() error: %v", err)
	}
	if err := client.DeclineMilestoneBundle(context.Background(), "token", "bundle-1"); err != nil {
		t.Fatalf("DeclineMilestoneBundle() error: %v", err)
	}
}
