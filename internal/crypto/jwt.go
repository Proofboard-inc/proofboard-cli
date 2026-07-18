package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtClaims struct {
	Exp int64 `json:"exp"`
}

func JWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("parse jwt expiry: invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse jwt expiry payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("parse jwt expiry claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("parse jwt expiry: exp missing")
	}

	return time.Unix(claims.Exp, 0).UTC(), nil
}
