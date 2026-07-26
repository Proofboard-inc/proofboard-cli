package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtClaims struct {
	Exp   int64  `json:"exp"`
	Scope string `json:"scope"`
}

func decodeJWTClaims(token string) (jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, fmt.Errorf("parse jwt claims: invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}, fmt.Errorf("parse jwt claims payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return jwtClaims{}, fmt.Errorf("parse jwt claims: %w", err)
	}
	return claims, nil
}

func JWTExpiry(token string) (time.Time, error) {
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse jwt expiry: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("parse jwt expiry: exp missing")
	}

	return time.Unix(claims.Exp, 0).UTC(), nil
}

func JWTScope(token string) (string, error) {
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return "", fmt.Errorf("parse jwt scope: %w", err)
	}
	if strings.TrimSpace(claims.Scope) == "" {
		return "", fmt.Errorf("parse jwt scope: scope missing")
	}
	return strings.TrimSpace(claims.Scope), nil
}
