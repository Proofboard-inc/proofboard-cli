package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/notifications"
)

var errAgentReconnectRequired = errors.New("proofboard career agent reconnect required")

const agentReconnectPromptInterval = 6 * time.Hour

func isAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "api returned 401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication required")
}

func runAuthFlow(ctx context.Context, out io.Writer) error {
	cmd := newAuthCommand(ctx, out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetErr(out)
	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("connect Career Agent: %w", err)
	}
	return nil
}

func loadOrAuthCredentials(ctx context.Context, out io.Writer, runtime runtimeContext) (model.Credentials, error) {
	credentials, err := runtime.credentials.Load(ctx)
	if err == nil && credentials.Token != "" && !credentialsNeedRefresh(credentials) {
		return credentials, nil
	}
	if err == nil && credentials.RefreshToken != "" {
		service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
		if refreshed, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
			return refreshed, nil
		}
	}
	if authErr := runAuthFlow(ctx, out); authErr != nil {
		return model.Credentials{}, authErr
	}
	credentials, err = runtime.credentials.Load(ctx)
	if err != nil {
		return model.Credentials{}, fmt.Errorf("reload credentials: %w", err)
	}
	if credentials.Token == "" {
		return model.Credentials{}, fmt.Errorf("career agent connection did not produce credentials")
	}
	return credentials, nil
}

func credentialsNeedRefresh(credentials model.Credentials) bool {
	expiresAt := credentialExpiry(credentials)
	return !expiresAt.IsZero() && time.Until(expiresAt) <= 5*time.Minute
}

func credentialsCompletelyExpired(credentials model.Credentials) bool {
	if credentials.RefreshToken != "" {
		return false
	}
	expiresAt := credentialExpiry(credentials)
	return !expiresAt.IsZero() && !time.Now().Before(expiresAt)
}

func credentialExpiry(credentials model.Credentials) time.Time {
	if !credentials.ExpiresAt.IsZero() {
		return credentials.ExpiresAt
	}
	if credentials.Token != "" {
		if parsed, err := crypto.JWTExpiry(credentials.Token); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func retryAfterAuth(ctx context.Context, out io.Writer, opName string, op func() error) error {
	err := op()
	if !isAuthFailure(err) {
		return err
	}
	if runtime, runtimeErr := loadRuntime(ctx); runtimeErr == nil {
		if credentials, loadErr := runtime.credentials.Load(ctx); loadErr == nil && credentials.RefreshToken != "" {
			service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
			if _, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
				if retryErr := op(); !isAuthFailure(retryErr) {
					return retryErr
				}
			}
		}
	}
	fmt.Fprintf(out, "Your Proofboard session has expired. Reconnecting to continue %s...\n", opName)
	if authErr := runAuthFlow(ctx, out); authErr != nil {
		return authErr
	}
	
	retryErr := op()
	if isAuthFailure(retryErr) {
		return fmt.Errorf("%w (device key may need rotation: run proofboard auth --rotate-key manually)", retryErr)
	}
	return retryErr
}

func retryAfterAuthForAgent(ctx context.Context, out io.Writer, runtime runtimeContext, op func() error) error {
	err := op()
	if !isAuthFailure(err) {
		return err
	}
	credentials, loadErr := runtime.credentials.Load(ctx)
	if loadErr == nil && credentials.RefreshToken != "" {
		service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
		if _, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
			if retryErr := op(); !isAuthFailure(retryErr) {
				return retryErr
			}
		} else if !isAuthFailure(refreshErr) {
			return refreshErr
		}
	}
	if err := promptAgentReconnect(ctx, out, runtime); err != nil {
		return err
	}
	return errAgentReconnectRequired
}

func promptAgentReconnect(ctx context.Context, out io.Writer, runtime runtimeContext) error {
	return promptAgentReconnectAt(ctx, out, runtime, time.Now())
}

func promptAgentReconnectAt(ctx context.Context, out io.Writer, runtime runtimeContext, now time.Time) error {
	current, err := runtime.state.Load(ctx)
	if err != nil {
		return err
	}
	if current.AuthReconnectPrompted &&
		!current.AuthReconnectPromptedAt.IsZero() &&
		now.Before(current.AuthReconnectPromptedAt.Add(agentReconnectPromptInterval)) {
		return nil
	}
	current.AuthReconnectPrompted = true
	current.AuthReconnectPromptedAt = now.UTC()
	if err := runtime.state.Save(ctx, current); err != nil {
		return err
	}
	notifications.PrintEvent(out, notifications.SessionExpired())
	_ = launchActionNotification(ctx, "reconnect")
	return nil
}
