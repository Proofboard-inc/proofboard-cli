package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMilestoneBundleAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/projects/milestone-bundles":
			if r.URL.Query().Get("status") != "pending" || r.URL.Query().Get("projectId") != "project-1" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"items": []map[string]any{{"id": "bundle-1", "title": "Payment Infrastructure Completed", "status": "pending"}},
				},
			})
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

	bundles, err := client.GetPendingMilestoneBundles(context.Background(), "token", "project-1", 5)
	if err != nil {
		t.Fatalf("GetPendingMilestoneBundles() error: %v", err)
	}
	if len(bundles) != 1 || bundles[0].ID != "bundle-1" || bundles[0].Title != "Payment Infrastructure Completed" {
		t.Fatalf("unexpected bundles: %+v", bundles)
	}
	if err := client.ApproveMilestoneBundle(context.Background(), "token", "bundle-1"); err != nil {
		t.Fatalf("ApproveMilestoneBundle() error: %v", err)
	}
	if err := client.DeclineMilestoneBundle(context.Background(), "token", "bundle-1"); err != nil {
		t.Fatalf("DeclineMilestoneBundle() error: %v", err)
	}
}
