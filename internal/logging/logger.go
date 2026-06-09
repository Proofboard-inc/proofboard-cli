package logging

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

func New(ctx context.Context, level string) (*zap.Logger, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	cfg := zap.NewProductionConfig()
	if level != "" {
		if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
			return nil, fmt.Errorf("parse log level: %w", err)
		}
	}
	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return logger, nil
}
