package api

import (
	"context"
	"fmt"
	"net/url"
)

func (c Client) ApproveMilestoneBundle(ctx context.Context, token, bundleID string) error {
	if bundleID == "" {
		return fmt.Errorf("approve milestone bundle: bundle id is required")
	}
	path := fmt.Sprintf("/api/v1/projects/milestone-bundles/%s/approve", url.PathEscape(bundleID))
	return c.postJSON(ctx, path, token, map[string]any{}, nil)
}

func (c Client) DeclineMilestoneBundle(ctx context.Context, token, bundleID string) error {
	if bundleID == "" {
		return fmt.Errorf("decline milestone bundle: bundle id is required")
	}
	path := fmt.Sprintf("/api/v1/projects/milestone-bundles/%s/decline", url.PathEscape(bundleID))
	return c.postJSON(ctx, path, token, nil, nil)
}
