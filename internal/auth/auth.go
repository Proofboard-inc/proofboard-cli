package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

type Service struct {
	store        CredentialStore
	client       api.Client
	agentAuthURL string
}

func NewService(store CredentialStore, client api.Client, agentAuthURL ...string) Service {
	authURL := "https://proofboard.io/agent/cli-auth"
	if len(agentAuthURL) > 0 && strings.TrimSpace(agentAuthURL[0]) != "" {
		authURL = agentAuthURL[0]
	}
	return Service{store: store, client: client, agentAuthURL: authURL}
}

func (s Service) Login(ctx context.Context, emailHash string) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("login: %w", err)
	}

	resp, err := s.client.CreateDeviceCode(ctx)
	if err != nil {
		return model.Credentials{}, err
	}
	if resp.DeviceCode == "" {
		return model.Credentials{}, fmt.Errorf("device-code response did not include a polling token")
	}
	if resp.UserCode == "" {
		return model.Credentials{}, fmt.Errorf("device-code response did not include a user code")
	}
	pollCtx := ctx
	if resp.ExpiresIn > 0 {
		var cancel context.CancelFunc
		pollCtx, cancel = context.WithTimeout(ctx, time.Duration(resp.ExpiresIn)*time.Second)
		defer cancel()
	}

	fmt.Printf("Connect your Proofboard Career Agent.\n\n")
	fmt.Printf("Your device code is: %s\n\n", resp.UserCode)

	verificationURL, err := s.resolveAuthorizationURL(ctx, resp.UserCode, resp.VerificationURL)
	if err != nil {
		return model.Credentials{}, err
	}
	if os.Getenv("NO_BROWSER") == "1" {
		fmt.Printf("Headless environment detected (NO_BROWSER=1).\nNavigate to the following URL manually to login:\n%s\n\n", verificationURL)
	} else if err := OpenBrowser(ctx, verificationURL); err != nil {
		fmt.Printf("Navigate to the following URL manually to login:\n%s\n\n", verificationURL)
	} else {
		fmt.Printf("Opening browser to connect your Career Agent...\nIf it does not open automatically, navigate to:\n%s\n\n", verificationURL)
	}
	fmt.Printf("Waiting for authentication...\n")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return model.Credentials{}, fmt.Errorf("authentication window closed: %w", pollCtx.Err())
		case <-ticker.C:
			pollResp, err := s.client.PollDeviceCode(pollCtx, resp.DeviceCode)
			if err != nil {
				// if it's a 429, we should just wait longer
				if strings.Contains(err.Error(), "429") {
					continue
				}
				continue // ignore temporary network errors during poll
			}
			if pollResp.Status == "approved" {
				creds := model.Credentials{
					Token:        pollResp.Token,
					RefreshToken: pollResp.RefreshToken,
					Username:     pollResp.Username,
					EmailHash:    emailHash,
				}
				if expiry, err := crypto.JWTExpiry(pollResp.Token); err == nil {
					creds.ExpiresAt = expiry
				}
				if creds.Username == "" {
					creds.Username = "Proofboard user"
				}
				if err := s.store.Save(ctx, creds); err != nil {
					return model.Credentials{}, fmt.Errorf("save credentials: %w", err)
				}
				return creds, nil
			} else if pollResp.Status == "expired" || pollResp.Status == "denied" {
				return model.Credentials{}, fmt.Errorf("authentication %s", pollResp.Status)
			}
		}
	}
}

func (s Service) resolveAuthorizationURL(ctx context.Context, userCode, fallbackURL string) (string, error) {
	preferredURL := s.authorizationURL(userCode)
	if authorizationPageAvailable(ctx, preferredURL) {
		return preferredURL, nil
	}
	fallbackURL = strings.TrimSpace(fallbackURL)
	if fallbackURL != "" && fallbackURL != preferredURL && authorizationPageAvailable(ctx, fallbackURL) {
		return fallbackURL, nil
	}
	return "", fmt.Errorf("proofboard authorization page is unavailable; try Reconnect again shortly")
}

func authorizationPageAvailable(ctx context.Context, candidateURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidateURL, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024))
	if err != nil {
		return false
	}
	lowerBody := strings.ToLower(string(body))
	for _, notFoundMarker := range []string{
		"this page could not be found",
		"next_http_error_fallback;404",
		"deployment_not_found",
	} {
		if strings.Contains(lowerBody, notFoundMarker) {
			return false
		}
	}
	return true
}

func (s Service) authorizationURL(deviceCode string) string {
	u, err := url.Parse(s.agentAuthURL)
	if err != nil {
		return s.agentAuthURL
	}
	q := u.Query()
	q.Set("code", deviceCode)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s Service) Refresh(ctx context.Context, credentials model.Credentials) (model.Credentials, error) {
	if credentials.RefreshToken == "" {
		return model.Credentials{}, fmt.Errorf("refresh credentials: refresh token is missing")
	}
	response, err := s.client.RefreshAccessToken(ctx, credentials.RefreshToken)
	if err != nil {
		return model.Credentials{}, fmt.Errorf("refresh credentials: %w", err)
	}
	credentials.Token = response.Token
	if response.RefreshToken != "" {
		credentials.RefreshToken = response.RefreshToken
	}
	if expiry, err := crypto.JWTExpiry(credentials.Token); err == nil {
		credentials.ExpiresAt = expiry
	}
	if err := s.store.Save(ctx, credentials); err != nil {
		return model.Credentials{}, fmt.Errorf("save refreshed credentials: %w", err)
	}
	return credentials, nil
}
