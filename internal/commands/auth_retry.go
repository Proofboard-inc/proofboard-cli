package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/proofboard/proofboard/internal/model"
)

func isAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "api returned 401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication required")
}

func runAuthFlow(ctx context.Context, out io.Writer) error {
	cmd := newAuthCommand(ctx, out)
	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("proofboard auth: %w", err)
	}
	return nil
}

func loadOrAuthCredentials(ctx context.Context, out io.Writer, runtime runtimeContext) (model.Credentials, error) {
	credentials, err := runtime.credentials.Load(ctx)
	if err == nil && credentials.Token != "" {
		return credentials, nil
	}
	if authErr := runAuthFlow(ctx, out); authErr != nil {
		return model.Credentials{}, authErr
	}
	credentials, err = runtime.credentials.Load(ctx)
	if err != nil {
		return model.Credentials{}, fmt.Errorf("reload credentials: %w", err)
	}
	if credentials.Token == "" {
		return model.Credentials{}, fmt.Errorf("proofboard auth did not produce credentials")
	}
	return credentials, nil
}

func retryAfterAuth(ctx context.Context, out io.Writer, opName string, op func() error) error {
	err := op()
	if !isAuthFailure(err) {
		return err
	}
	fmt.Fprintf(out, "Authentication expired while running %s. Re-authenticating...\n", opName)
	if authErr := runAuthFlow(ctx, out); authErr != nil {
		return authErr
	}
	return op()
}
