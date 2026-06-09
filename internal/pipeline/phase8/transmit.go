package phase8

import (
	"context"
	"fmt"
	"net/url"

	"github.com/proofboard/proofboard/internal/model"
)

func Transmit(ctx context.Context, endpoint string, payload model.SyncPayload) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("transmit payload: %w", err)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse API endpoint: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("API endpoint must use HTTPS")
	}
	return nil
}
