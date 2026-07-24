package api

import (
	"context"
	"fmt"
	"net/http"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
	ExpiresIn       int    `json:"expiresIn"`
}

type PollDeviceCodeResponse struct {
	Status       string `json:"status"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Username     string `json:"username"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type RefreshTokenResponse struct {
	Token        string `json:"token"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type DeviceKeyRegistrationRequest struct {
	PublicKey string `json:"publicKey"`
}

type DeviceKeyRegistrationResponse struct {
	DeviceKeyID string `json:"deviceKeyId"`
}

func (c Client) CreateDeviceCode(ctx context.Context) (DeviceCodeResponse, error) {
	var parsed DeviceCodeResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/api/v1/cli/auth/device-code", "", nil, map[string]string{}, &parsed); err != nil {
		return DeviceCodeResponse{}, err
	}
	return parsed, nil
}

func (c Client) PollDeviceCode(ctx context.Context, deviceCode string) (PollDeviceCodeResponse, error) {
	var parsed PollDeviceCodeResponse
	if err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/cli/auth/poll/%s", deviceCode), "", nil, nil, &parsed); err != nil {
		return PollDeviceCodeResponse{}, err
	}
	return parsed, nil
}

func (c Client) RegisterDeviceKey(ctx context.Context, token string, publicKey string) (DeviceKeyRegistrationResponse, error) {
	path := c.deviceKeyRegistrationPath
	if path == "" {
		path = "/api/v1/cli/auth/device-key"
	}
	var parsed DeviceKeyRegistrationResponse
	if err := c.requestJSON(ctx, http.MethodPost, path, token, nil, DeviceKeyRegistrationRequest{PublicKey: publicKey}, &parsed); err != nil {
		return DeviceKeyRegistrationResponse{}, err
	}
	return parsed, nil
}

func (c Client) RefreshAccessToken(ctx context.Context, refreshToken string) (RefreshTokenResponse, error) {
	path := c.refreshPath
	if path == "" {
		path = "/api/v1/cli/auth/refresh"
	}
	var parsed RefreshTokenResponse
	if err := c.requestJSON(ctx, http.MethodPost, path, "", nil, RefreshTokenRequest{RefreshToken: refreshToken}, &parsed); err != nil {
		return RefreshTokenResponse{}, err
	}
	if parsed.Token == "" {
		parsed.Token = parsed.AccessToken
	}
	if parsed.Token == "" {
		return RefreshTokenResponse{}, fmt.Errorf("refresh response did not include an access token")
	}
	return parsed, nil
}
