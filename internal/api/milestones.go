package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/proofboard/proofboard/internal/model"
)

func (c Client) GetPendingMilestoneBundles(ctx context.Context, token, projectID string, limit int) ([]model.MilestoneBundle, error) {
	query := url.Values{}
	query.Set("status", "pending")
	if projectID != "" {
		query.Set("projectId", projectID)
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	var raw json.RawMessage
	if err := c.requestJSON(ctx, http.MethodGet, "/api/v1/projects/milestone-bundles", token, query, nil, &raw); err != nil {
		return nil, err
	}
	return decodeMilestoneBundles(raw)
}

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

func decodeMilestoneBundles(raw json.RawMessage) ([]model.MilestoneBundle, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode milestone bundles: %w", err)
	}
	bundles := make([]model.MilestoneBundle, 0)
	seen := make(map[string]bool)
	collectMilestoneBundles(value, &bundles, seen)
	return bundles, nil
}

func collectMilestoneBundles(value any, bundles *[]model.MilestoneBundle, seen map[string]bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectMilestoneBundles(item, bundles, seen)
		}
	case map[string]any:
		id := firstString(typed, "id", "_id", "bundleId", "milestoneBundleId")
		if id != "" && !seen[id] && looksLikeMilestoneBundle(typed) {
			seen[id] = true
			*bundles = append(*bundles, model.MilestoneBundle{
				ID:             id,
				Title:          firstString(typed, "title", "name"),
				OutcomeSummary: firstString(typed, "outcomeSummary", "summary"),
				Category:       firstString(typed, "category"),
				Status:         firstString(typed, "status"),
				ProjectID:      firstString(typed, "projectId"),
			})
		}
		for _, child := range typed {
			collectMilestoneBundles(child, bundles, seen)
		}
	}
}

func looksLikeMilestoneBundle(value map[string]any) bool {
	for _, key := range []string{"title", "outcomeSummary", "category", "projectId", "commitCount", "milestoneBundleId", "bundleId"} {
		if _, ok := value[key]; ok {
			return true
		}
	}
	return false
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if item, ok := value[key].(string); ok && item != "" {
			return item
		}
	}
	return ""
}
