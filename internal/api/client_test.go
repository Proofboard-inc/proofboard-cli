package api

import (
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

func TestRedactJSONForLogRemovesCredentialsRecursively(t *testing.T) {
	input := []byte(`{"token":"access-secret","refreshToken":"refresh-secret","nested":{"deviceSignature":"signature-secret"},"status":"approved"}`)
	got := redactJSONForLog(input)
	for _, secret := range []string{"access-secret", "refresh-secret", "signature-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted log contains %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, `"status":"approved"`) {
		t.Fatalf("expected non-sensitive status to remain: %s", got)
	}
}
