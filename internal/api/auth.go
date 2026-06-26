package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"deviceCode"`
	VerificationURL string `json:"verificationUrl"`
	ExpiresIn       int    `json:"expiresIn"`
}

type PollDeviceCodeResponse struct {
	Status       string `json:"status"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Username     string `json:"username"`
}

func (c Client) CreateDeviceCode(ctx context.Context, deviceCode string) (DeviceCodeResponse, error) {
	reqBody, _ := json.Marshal(map[string]string{"deviceCode": deviceCode})
	url := fmt.Sprintf("%s/api/v1/cli/auth/device-code", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return DeviceCodeResponse{}, fmt.Errorf("server returned %s", resp.Status)
	}

	var parsed struct {
		Success bool               `json:"success"`
		Data    DeviceCodeResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return parsed.Data, nil
}

func (c Client) PollDeviceCode(ctx context.Context, deviceCode string) (PollDeviceCodeResponse, error) {
	url := fmt.Sprintf("%s/api/v1/cli/auth/poll/%s", c.baseURL, deviceCode)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return PollDeviceCodeResponse{}, fmt.Errorf("create poll request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PollDeviceCodeResponse{}, fmt.Errorf("do poll request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PollDeviceCodeResponse{}, fmt.Errorf("poll returned %s", resp.Status)
	}

	var parsed struct {
		Success bool                   `json:"success"`
		Data    PollDeviceCodeResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return PollDeviceCodeResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return parsed.Data, nil
}
