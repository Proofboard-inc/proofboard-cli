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



func TestGetNotificationsReal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notifications" {
			t.Errorf("expected path /api/v1/notifications, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1 query parameter, got %s", r.URL.Query().Get("page"))
		}
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			t.Errorf("expected Bearer mock-token, got %s", r.Header.Get("Authorization"))
		}
		resp := model.PaginatedNotifications{
			Data: []model.Notification{
				{
					ID:        "notif-1",
					Type:      "vcs_sync_completed",
					IsRead:    false,
					ActionURL: "https://proofboard.io/notif-1",
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

	client := NewClient(server.URL, "/cli/link", "/cli/sync")
	query := url.Values{}
	query.Set("page", "1")
	res, err := client.GetNotifications(context.Background(), "mock-token", query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Data) != 1 || res.Data[0].ID != "notif-1" {
		t.Fatalf("expected 1 notification with ID notif-1, got %+v", res)
	}
}

func TestGetUnreadNotificationCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notifications/unread-count" {
			t.Errorf("expected path /api/v1/notifications/unread-count, got %s", r.URL.Path)
		}
		resp := model.UnreadCountResponse{Count: 5}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "/cli/link", "/cli/sync")
	res, err := client.GetUnreadNotificationCount(context.Background(), "mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Count != 5 {
		t.Fatalf("expected count 5, got %d", res.Count)
	}
}

func TestMarkNotificationRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notifications/notif-1/read" {
			t.Errorf("expected path /api/v1/notifications/notif-1/read, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "/cli/link", "/cli/sync")
	err := client.MarkNotificationRead(context.Background(), "mock-token", "notif-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkAllNotificationsRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notifications/mark-all-read" {
			t.Errorf("expected path /api/v1/notifications/mark-all-read, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "/cli/link", "/cli/sync")
	err := client.MarkAllNotificationsRead(context.Background(), "mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
