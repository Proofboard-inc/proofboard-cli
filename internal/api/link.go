package api

import (
	"context"
)

type LinkHandshake struct {
	SSHTest bool `json:"sshTest"`
}

type LinkRequest struct {
	OrgHash           string         `json:"orgHash"`
	RepoHash          string         `json:"repoHash"`
	Provider          string         `json:"provider"`
	ExistingProjectID string         `json:"existingProjectId,omitempty"`
	CreateNew         bool           `json:"createNew,omitempty"`
	Handshake         *LinkHandshake `json:"handshake,omitempty"`
}

type ExistingProjectOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type LinkResponse struct {
	IsNewProject           bool                    `json:"isNewProject"`
	ProjectID              string                  `json:"projectId"`
	ExistingProjectOptions []ExistingProjectOption `json:"existingProjectOptions"`
	DictionaryVersion      string                  `json:"dictionaryVersion"`
	PublicKey              string                  `json:"publicKey"`
}

func (c Client) Link(ctx context.Context, token string, req LinkRequest) (LinkResponse, error) {
	var response LinkResponse
	err := c.postJSON(ctx, c.linkPath, token, req, &response)
	return response, err
}
