package model

type RemoteIdentity struct {
	Provider string
	Org      string
	Repo     string
	OrgHash  string
	RepoHash string
}
