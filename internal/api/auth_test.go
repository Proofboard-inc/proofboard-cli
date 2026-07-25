package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDeviceCodeUsesServerGeneratedContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cli/auth/device-code" {
			http.NotFound(w, r)
			return
		}
		if r.Body != nil && r.ContentLength > 0 {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if _, exists := payload["deviceCode"]; exists {
				t.Fatal("client sent forbidden deviceCode property")
			}
		}
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "secret-polling-token",
			UserCode:        "ABCD-1234",
			VerificationURL: "https://proofboard.io/agent/cli-auth?code=ABCD-1234",
			ExpiresIn:       600,
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "", "", "")
	response, err := client.CreateDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("CreateDeviceCode() error: %v", err)
	}
	if response.DeviceCode != "secret-polling-token" || response.UserCode != "ABCD-1234" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
