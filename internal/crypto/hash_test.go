package crypto

import (
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
