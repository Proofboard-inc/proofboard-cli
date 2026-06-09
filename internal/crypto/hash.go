package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func SHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func NormalizedSHA256(value string) string {
	return SHA256(strings.ToLower(strings.TrimSpace(value)))
}
