package api

import "context"

func (c Client) Authenticate(ctx context.Context) error {
	return ctx.Err()
}
