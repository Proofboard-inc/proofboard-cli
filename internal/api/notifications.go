package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/proofboard/proofboard/internal/model"
)

func (c Client) GetNotifications(ctx context.Context, token string, query url.Values) (model.PaginatedNotifications, error) {
	var response model.PaginatedNotifications
	err := c.getJSON(ctx, "/api/v1/notifications", token, query, &response)
	return response, err
}

func (c Client) GetUnreadNotificationCount(ctx context.Context, token string) (model.UnreadCountResponse, error) {
	var response model.UnreadCountResponse
	err := c.getJSON(ctx, "/api/v1/notifications/unread-count", token, nil, &response)
	return response, err
}

func (c Client) MarkNotificationRead(ctx context.Context, token string, id string) error {
	path := fmt.Sprintf("/api/v1/notifications/%s/read", id)
	return c.patchJSON(ctx, path, token, nil, nil)
}

func (c Client) MarkAllNotificationsRead(ctx context.Context, token string) error {
	return c.patchJSON(ctx, "/api/v1/notifications/mark-all-read", token, nil, nil)
}
