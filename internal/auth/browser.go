package auth

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

func OpenBrowser(ctx context.Context, target string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	if _, err := url.ParseRequestURI(target); err != nil {
		return fmt.Errorf("parse browser URL: %w", err)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", target)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
