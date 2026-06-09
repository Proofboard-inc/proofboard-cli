package phase6

import (
	"context"
	"fmt"
)

func Handshake(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	return nil
}
