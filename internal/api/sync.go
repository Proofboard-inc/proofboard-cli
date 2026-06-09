package api

import (
	"context"

	"github.com/proofboard/proofboard/internal/model"
)

func (c Client) Sync(ctx context.Context, token string, payload model.SyncPayload) (model.SyncReceipt, error) {
	var receipt model.SyncReceipt
	err := c.postJSON(ctx, c.syncPath, token, payload, &receipt)
	return receipt, err
}
