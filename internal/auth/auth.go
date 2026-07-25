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

	pollTimer := time.NewTimer(0)
	defer pollTimer.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return model.Credentials{}, fmt.Errorf("authentication window closed: %w", pollCtx.Err())
		case <-pollTimer.C:
			pollResp, err := s.client.PollDeviceCode(pollCtx, resp.DeviceCode)
			if err != nil {
				pollTimer.Reset(5 * time.Second)
				continue
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
			pollTimer.Reset(5 * time.Second)
		}
	}
}

// resolveAuthorizationURL returns the page the engineer is sent to. The
// configured address is used as given: a reachability check must never stand
// between someone and connecting their Career Agent, so an unreachable page is
// reported and the address is still handed over.
func (s Service) resolveAuthorizationURL(ctx context.Context, userCode, _ string) (string, error) {
	authorizationURL := s.authorizationURL(userCode)
	if !authorizationPageAvailable(ctx, authorizationURL) {
		fmt.Printf("The Proofboard authorization page did not respond. Open this address to finish connecting:\n%s\n\n", authorizationURL)
	}
	return authorizationURL, nil
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
	return !pageIsNotFound(string(body))
}

// pageIsNotFound recognises a page that answered 200 but is really a "not
// found" page. Only the document title is examined: single page applications
// ship their not-found text inside every page they serve, so searching the
// whole document rejects working pages.
func pageIsNotFound(body string) bool {
	title := documentTitle(body)
	if title == "" {
		return strings.Contains(strings.ToLower(body), "deployment_not_found")
	}
	for _, marker := range []string{
		"this page could not be found",
		"404",
		"not found",
		"deployment_not_found",
	} {
		if strings.Contains(title, marker) {
			return true
		}
	}
	return false
}

func documentTitle(body string) string {
	lower := strings.ToLower(body)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	open := strings.Index(lower[start:], ">")
	if open < 0 {
		return ""
	}
	start += open + 1
	end := strings.Index(lower[start:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(lower[start : start+end])
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
