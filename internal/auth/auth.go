package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/model"
)

type Service struct {
	store  CredentialStore
	client api.Client
}

func NewService(store CredentialStore, client api.Client) Service {
	return Service{store: store, client: client}
}

func generateDeviceCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	hexStr := strings.ToUpper(hex.EncodeToString(b))
	return hexStr[:4] + "-" + hexStr[4:]
}

func (s Service) Login(ctx context.Context, emailHash string) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("login: %w", err)
	}

	code := generateDeviceCode()
	resp, err := s.client.CreateDeviceCode(ctx, code)
	if err != nil {
		return model.Credentials{}, err
	}

	fmt.Printf("Please authenticate your CLI session.\n\n")
	fmt.Printf("Your device code is: %s\n\n", resp.DeviceCode)

	if os.Getenv("NO_BROWSER") == "1" {
		fmt.Printf("Headless environment detected (NO_BROWSER=1).\nNavigate to the following URL manually to login:\n%s\n\n", resp.VerificationURL)
	} else if err := OpenBrowser(ctx, resp.VerificationURL); err != nil {
		fmt.Printf("Navigate to the following URL manually to login:\n%s\n\n", resp.VerificationURL)
	} else {
		fmt.Printf("Opening browser to complete authentication...\nIf it does not open automatically, navigate to:\n%s\n\n", resp.VerificationURL)
	}
	fmt.Printf("Waiting for authentication...\n")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return model.Credentials{}, ctx.Err()
		case <-ticker.C:
			pollResp, err := s.client.PollDeviceCode(ctx, resp.DeviceCode)
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
