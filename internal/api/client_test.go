package api

import "testing"

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
