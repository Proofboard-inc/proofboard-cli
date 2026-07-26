package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func SHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func NormalizedSHA256(value string) string {
	return SHA256(strings.ToLower(strings.TrimSpace(value)))
}

// NormalizedHMACSHA256 applies the per-project 32-byte key returned by the
// link endpoint to a normalized identity value. The key is encoded as exactly
// 64 hexadecimal characters by the service.
func NormalizedHMACSHA256(keyHex, value string) (string, error) {
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return "", fmt.Errorf("decode HMAC key: %w", err)
	}
	if len(key) != sha256.Size {
		return "", fmt.Errorf("HMAC key must be %d bytes, got %d", sha256.Size, len(key))
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
