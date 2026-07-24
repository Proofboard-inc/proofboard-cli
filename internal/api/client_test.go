package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndpointRejectsNonHTTPSOutsideLocalhost(t *testing.T) {
	t.Parallel()
	client := NewClient("http://api.example.com", "/cli/link", "/cli/check", "/cli/sync")
	if _, err := client.endpoint("/cli/sync"); err == nil {
		t.Fatalf("expected non-HTTPS endpoint to be rejected")
	}
}

func TestEndpointAllowsLocalhostHTTPForTests(t *testing.T) {
	t.Parallel()
	client := NewClient("http://127.0.0.1:1234", "/cli/link", "/cli/check", "/cli/sync")
	if _, err := client.endpoint("/cli/sync"); err != nil {
		t.Fatalf("expected localhost endpoint to be allowed: %v", err)
	}
}

func TestRedactJSONForLogKeepsOnlyNumericStatusCode(t *testing.T) {
	input := []byte(`{"token":"access-secret","refreshToken":"refresh-secret","nested":{"deviceSignature":"signature-secret"},"status":"approved","statusCode":200,"projectName":"Confidential Payments","message":"Repository Confidential Payments is linked"}`)
	got := redactJSONForLog(input)
	for _, secret := range []string{"access-secret", "refresh-secret", "signature-secret", "Confidential Payments", "Repository", "approved"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted log contains %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, `"statusCode":200`) {
		t.Fatalf("expected safe numeric status code to remain: %s", got)
	}
}

func TestClientNeverPersistsRequestOrResponseBodies(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"projectName":"Secret Payments","organization":"Secret Employer","status":"ok"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "/link", "/check", "/sync")
	var response map[string]any
	if err := client.requestJSON(
		context.Background(),
		http.MethodPost,
		"/link",
		"access-token",
		nil,
		map[string]string{"repositoryName": "Secret Payments"},
		&response,
	); err != nil {
		t.Fatalf("requestJSON: %v", err)
	}

	logPath := filepath.Join(homeDir, ".proofboard", "sync.log")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("HTTP client persisted API bodies to %s: %v", logPath, err)
	}
}

func TestAPIErrorNeverIncludesProprietaryResponseText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":400,"message":"Repository Secret Payments at Secret Employer failed"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "/link", "/check", "/sync")
	err := client.requestJSON(context.Background(), http.MethodPost, "/link", "", nil, map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	for _, secret := range []string{"Secret Payments", "Secret Employer", "Repository"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("API error retained proprietary response text %q: %v", secret, err)
		}
	}
	if !strings.Contains(err.Error(), `"statusCode":400`) {
		t.Fatalf("API error lost safe status metadata: %v", err)
	}
}
