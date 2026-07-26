package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

func (c Client) Sync(ctx context.Context, token string, payload model.SyncPayload) (model.SyncReceipt, error) {
	var response struct {
		model.SyncReceipt
		Success bool              `json:"success"`
		Data    model.SyncReceipt `json:"data"`
	}
	for attempt := 0; attempt < 3; attempt++ {
		err := c.postJSON(ctx, c.syncPath, token, payload, &response)
		if err == nil {
			receipt := response.SyncReceipt
			if response.Data.ID != "" || response.Data.SyncID != "" || response.Data.Status != "" {
				receipt = response.Data
			}
			if receipt.ID == "" {
				receipt.ID = receipt.SyncID
			}
			return receipt, nil
		}
		if IsStatus(err, http.StatusConflict) {
			return model.SyncReceipt{Status: "duplicate", Message: "Duplicate payload ignored safely."}, nil
		}
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests || attempt == 2 {
			return model.SyncReceipt{}, err
		}
		delay := apiErr.RetryAfter
		if delay <= 0 {
			delay = time.Duration(attempt+1) * time.Second
		}
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return model.SyncReceipt{}, ctx.Err()
		case <-timer.C:
		}
	}
	return model.SyncReceipt{}, errors.New("sync retry limit exhausted")
}
