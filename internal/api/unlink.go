package api

import (
	"context"
	"fmt"
)

type UnlinkRepoResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// UnlinkRepo tells the backend to deregister repoHash: DELETE /api/v1/cli/repos/:repoHash.
// A 404 means the backend already considers this repo unlinked (or it was never
// registered server-side) — callers should treat that as success, not a warning.
func (c Client) UnlinkRepo(ctx context.Context, token string, repoHash string) (UnlinkRepoResponse, error) {
	var response UnlinkRepoResponse
	path := fmt.Sprintf("/api/v1/cli/repos/%s", repoHash)
	err := c.deleteJSON(ctx, path, token, &response)
	return response, err
}
