package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
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
	if api.IsStatus(err, 401) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "api returned 401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication required")
}

func isRevokedDeviceKey(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		detail := strings.ToLower(strings.TrimSpace(apiErr.Code + " " + apiErr.Message))
		return (strings.Contains(detail, "device") && strings.Contains(detail, "key")) &&
			(strings.Contains(detail, "revoked") || strings.Contains(detail, "unknown"))
	}
	detail := strings.ToLower(err.Error())
	return (strings.Contains(detail, "device") && strings.Contains(detail, "key")) &&
		(strings.Contains(detail, "revoked") || strings.Contains(detail, "unknown"))
}

func runAuthFlow(ctx context.Context, out io.Writer, rotateKey bool) error {
	cmd := newAuthCommand(ctx, out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetErr(out)
	// --force is required here. This runs because the server rejected the
	// current credentials, so "you already have credentials" is precisely the
	// wrong answer; without it the command returns success without signing in
	// and the caller retries with the same rejected token.
	args := []string{"--force"}
	if rotateKey {
		args = append(args, "--rotate-key")
	}
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("connect Career Agent: %w", err)
	}
	return nil
}

// runLinkFlow connects the current repository without prompting. It exists so
// a failed sync can repair itself: the backend rejects a payload for a
// repository the account has no project for, and the CLI is holding working
// credentials at that moment, so the connection it needs is one it can make.
func runLinkFlow(ctx context.Context, out io.Writer) error {
	cmd := newLinkCommand(ctx, out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetErr(out)
	// --non-interactive because this runs mid-sync, often from the background
	// agent where there is no terminal to answer a prompt.
	cmd.SetArgs([]string{"--non-interactive"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("connect this repository: %w", err)
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
	if authErr := runAuthFlow(ctx, out, false); authErr != nil {
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

func rotateDeviceKey(ctx context.Context, runtime runtimeContext) error {
	credentials, err := runtime.credentials.Load(ctx)
	if err != nil {
		return fmt.Errorf("load credentials for device-key rotation: %w", err)
	}
	if credentials.Token == "" {
		return errors.New("authentication required for device-key rotation")
	}
	record, err := pbauth.NewDeviceKeyStore(runtime.homeDir).Ensure(ctx, runtime.api, credentials.Token, true)
	if err != nil {
		return fmt.Errorf("rotate device key: %w", err)
	}
	credentials.DeviceKeyID = record.DeviceKeyID
	if err := runtime.credentials.Save(ctx, credentials); err != nil {
		return fmt.Errorf("persist rotated device key: %w", err)
	}
	return nil
}

// retryRevokedDeviceKey rotates the installation key with the currently
// stored access token and retries the operation once. It never launches an
// interactive authentication flow.
func retryRevokedDeviceKey(ctx context.Context, runtime runtimeContext, op func() error, err error) (error, bool) {
	if !isRevokedDeviceKey(err) {
		return err, false
	}
	if rotateErr := rotateDeviceKey(ctx, runtime); rotateErr != nil {
		return rotateErr, true
	}
	return op(), true
}

func retryAfterAuth(ctx context.Context, out io.Writer, opName string, op func() error) error {
	err := op()
	if !isAuthFailure(err) {
		return err
	}
	runtime, runtimeErr := loadRuntime(ctx)
	if runtimeErr == nil {
		if retryErr, handled := retryRevokedDeviceKey(ctx, runtime, op, err); handled {
			if !isAuthFailure(retryErr) {
				return retryErr
			}
			err = retryErr
		}
		if credentials, loadErr := runtime.credentials.Load(ctx); loadErr == nil && credentials.RefreshToken != "" {
			service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
			if _, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
				retryErr := op()
				if !isAuthFailure(retryErr) {
					return retryErr
				}
				if rotatedRetryErr, handled := retryRevokedDeviceKey(ctx, runtime, op, retryErr); handled {
					if !isAuthFailure(rotatedRetryErr) {
						return rotatedRetryErr
					}
					return fmt.Errorf("%s rejected refreshed credentials: %w", opName, rotatedRetryErr)
				}
				// A fresh access token that works for the refresh endpoint but
				// is still rejected by this operation is not an expired
				// session. Starting device auth here creates a login loop and
				// cannot repair an endpoint-specific policy or payload error.
				return fmt.Errorf("%s rejected refreshed credentials: %w", opName, retryErr)
			} else if !isAuthFailure(refreshErr) {
				return refreshErr
			}
		}
	}
	fmt.Fprintf(out, "Your Proofboard session has expired. Reconnecting to continue %s...\n", opName)
	if authErr := runAuthFlow(ctx, out, isRevokedDeviceKey(err)); authErr != nil {
		return authErr
	}

	retryErr := op()
	if isRevokedDeviceKey(retryErr) && runtimeErr == nil {
		if rotatedRetryErr, handled := retryRevokedDeviceKey(ctx, runtime, op, retryErr); handled {
			retryErr = rotatedRetryErr
		}
	}
	if isAuthFailure(retryErr) {
		return fmt.Errorf("%w after one reconnect attempt", retryErr)
	}
	return retryErr
}

func retryAfterAuthForAgent(ctx context.Context, out io.Writer, runtime runtimeContext, op func() error) error {
	err := op()
	if !isAuthFailure(err) {
		return err
	}
	if retryErr, handled := retryRevokedDeviceKey(ctx, runtime, op, err); handled {
		if !isAuthFailure(retryErr) {
			return retryErr
		}
		err = retryErr
	}
	credentials, loadErr := runtime.credentials.Load(ctx)
	if loadErr == nil && credentials.RefreshToken != "" {
		service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
		if _, refreshErr := service.Refresh(ctx, credentials); refreshErr == nil {
			retryErr := op()
			if !isAuthFailure(retryErr) {
				return retryErr
			}
			if rotatedRetryErr, handled := retryRevokedDeviceKey(ctx, runtime, op, retryErr); handled {
				if !isAuthFailure(rotatedRetryErr) {
					return rotatedRetryErr
				}
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
