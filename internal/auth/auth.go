package auth

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/proofboard/proofboard/internal/model"
)

type Service struct {
	store      CredentialStore
	appBaseURL string
	port       int
}

func NewService(store CredentialStore, appBaseURL string, port int) Service {
	return Service{store: store, appBaseURL: appBaseURL, port: port}
}

func (s Service) Login(ctx context.Context, emailHash string) (model.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return model.Credentials{}, fmt.Errorf("login: %w", err)
	}
	callback := CallbackServer{Port: s.port}
	authURL, err := s.authURL(emailHash)
	if err != nil {
		return model.Credentials{}, err
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		credentials model.Credentials
		err         error
	}
	done := make(chan result, 1)
	go func() {
		credentials, err := callback.Wait(waitCtx)
		done <- result{credentials: credentials, err: err}
	}()
	if err := OpenBrowser(ctx, authURL); err != nil {
		return model.Credentials{}, err
	}
	authResult := <-done
	if authResult.err != nil {
		return model.Credentials{}, authResult.err
	}
	if authResult.credentials.EmailHash == "" {
		authResult.credentials.EmailHash = emailHash
	}
	if err := s.store.Save(ctx, authResult.credentials); err != nil {
		return model.Credentials{}, err
	}
	return authResult.credentials, nil
}

func (s Service) authURL(emailHash string) (string, error) {
	base, err := url.Parse(s.appBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse app base URL: %w", err)
	}
	path, err := url.Parse("/cli-auth")
	if err != nil {
		return "", fmt.Errorf("parse auth path: %w", err)
	}
	authURL := base.ResolveReference(path)
	query := authURL.Query()
	query.Set("port", strconv.Itoa(s.port))
	query.Set("emailHash", emailHash)
	authURL.RawQuery = query.Encode()
	return authURL.String(), nil
}
