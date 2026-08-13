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

// GetCliNotifications and MarkCliNotificationRead are the CLI-JWT-scoped
// equivalents of GetNotifications/MarkNotificationRead above. The CLI's
// device-code-issued token is a completely separate auth strategy from the
// user session JWT (see backend CLAUDE.md), so it cannot call
// /api/v1/notifications: that route only accepts a user session token.
// /api/v1/cli/notifications is the CLI-guarded mirror that lets
// `proofboard notices` work for real (device-code-authenticated) CLI installs.
func (c Client) GetCliNotifications(ctx context.Context, token string, query url.Values) (model.PaginatedNotifications, error) {
	var response model.PaginatedNotifications
	err := c.getJSON(ctx, "/api/v1/cli/notifications", token, query, &response)
	return response, err
}

func (c Client) MarkCliNotificationRead(ctx context.Context, token string, id string) error {
	path := fmt.Sprintf("/api/v1/cli/notifications/%s/read", id)
	return c.patchJSON(ctx, path, token, nil, nil)
}
