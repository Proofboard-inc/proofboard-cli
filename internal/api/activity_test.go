package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func TestGetActivityLogReal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/activity-log" {
			t.Errorf("expected path /api/v1/activity-log, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "vcs_synced" {
			t.Errorf("expected type=vcs_synced query parameter, got %s", r.URL.Query().Get("type"))
		}
		resp := model.PaginatedActivityLogs{
			Data: []model.ActivityLog{
				{
					ID:        "act-1",
					Type:      "vcs_synced",
					CreatedAt: time.Now(),
				},
			},
			Meta: model.PaginationMeta{
				Total:      1,
				Page:       1,
				Limit:      10,
				TotalPages: 1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "/cli/link", "/cli/check", "/cli/sync")
	query := url.Values{}
	query.Set("type", "vcs_synced")
	res, err := client.GetActivityLog(context.Background(), "mock-token", query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Data) != 1 || res.Data[0].ID != "act-1" {
		t.Fatalf("expected 1 activity log with ID act-1, got %+v", res)
	}
}
