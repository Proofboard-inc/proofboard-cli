package api

import (
	"context"
	"net/url"

	"github.com/proofboard/proofboard/internal/model"
)

func (c Client) GetActivityLog(ctx context.Context, token string, query url.Values) (model.PaginatedActivityLogs, error) {
	var response model.PaginatedActivityLogs
	err := c.getJSON(ctx, "/api/v1/activity-log", token, query, &response)
	return response, err
}
