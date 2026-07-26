package crypto

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestJWTExpiry(t *testing.T) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"exp": 1719500000})
	token := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	expiry, err := JWTExpiry(token)
	if err != nil {
		t.Fatalf("JWTExpiry returned error: %v", err)
	}

	expected := time.Unix(1719500000, 0).UTC()
	if !expiry.Equal(expected) {
		t.Fatalf("JWTExpiry = %v, want %v", expiry, expected)
	}
}

func TestJWTScope(t *testing.T) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"exp": 1719500000, "scope": "cli"})
	token := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	scope, err := JWTScope(token)
	if err != nil {
		t.Fatalf("JWTScope returned error: %v", err)
	}
	if scope != "cli" {
		t.Fatalf("JWTScope = %q, want cli", scope)
	}
}
