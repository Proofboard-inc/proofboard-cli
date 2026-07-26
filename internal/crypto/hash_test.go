package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSHA256(t *testing.T) {
	hash := SHA256("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" // echo -n "test" | sha256sum
	if hash != expected {
		t.Errorf("expected %s, got %s", expected, hash)
	}
}

func TestNormalizedSHA256(t *testing.T) {
	hash1 := NormalizedSHA256("Test@Example.com ")
	hash2 := NormalizedSHA256("test@example.com")

	if hash1 != hash2 {
		t.Errorf("expected normalized hashes to match, got %s and %s", hash1, hash2)
	}
}

func TestNormalizedHMACSHA256(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, err := NormalizedHMACSHA256(keyHex, " Engineer@Example.com ")
	if err != nil {
		t.Fatalf("NormalizedHMACSHA256: %v", err)
	}
	key, _ := hex.DecodeString(keyHex)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(strings.ToLower("Engineer@Example.com")))
	want := hex.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Fatalf("HMAC = %q, want %q", got, want)
	}
}

func TestNormalizedHMACSHA256RejectsInvalidKey(t *testing.T) {
	for _, key := range []string{"not-hex", "abcd"} {
		if _, err := NormalizedHMACSHA256(key, "engineer@example.com"); err == nil {
			t.Fatalf("expected invalid key %q to fail", key)
		}
	}
}
