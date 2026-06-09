package api

import (
	"context"

	"github.com/proofboard/proofboard/internal/model"
)

type LinkRequest struct {
	OrgHash   string `json:"orgHash"`
	RepoHash  string `json:"repoHash"`
	EmailHash string `json:"emailHash"`
}

type LinkResponse struct {
	DisplayOrg string `json:"displayOrg"`
	Tier       string `json:"tier"`
}

func (c Client) Link(ctx context.Context, token string, identity model.RemoteIdentity, emailHash string) (LinkResponse, error) {
	var response LinkResponse
	err := c.postJSON(ctx, c.linkPath, token, LinkRequest{
		OrgHash:   identity.OrgHash,
		RepoHash:  identity.RepoHash,
		EmailHash: emailHash,
	}, &response)
	return response, err
}
