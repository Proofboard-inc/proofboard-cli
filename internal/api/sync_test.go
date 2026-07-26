package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func TestSyncDecodesAcceptedResponseEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]string{
				"syncId":  "sync-123",
				"status":  "accepted",
				"message": "Payload accepted.",
			},
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "", "", "/sync")
	receipt, err := client.Sync(context.Background(), "token", model.SyncPayload{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if receipt.ID != "sync-123" || receipt.Status != "accepted" {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestSyncTreatsDuplicateAsSafeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "DUPLICATE_PAYLOAD"})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "", "", "/sync")
	receipt, err := client.Sync(context.Background(), "token", model.SyncPayload{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if receipt.Status != "duplicate" {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestSyncBacksOffAndRetriesRateLimit(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "RATE_LIMITED"})
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(model.SyncReceipt{ID: "sync-after-retry", Status: "accepted"})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "", "", "/sync")
	receipt, err := client.Sync(context.Background(), "token", model.SyncPayload{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if calls != 3 || receipt.ID != "sync-after-retry" {
		t.Fatalf("calls = %d, receipt = %#v", calls, receipt)
	}
}
