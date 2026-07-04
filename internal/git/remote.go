package git

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

type Remote struct {
	URL string
}

func OriginURL(ctx context.Context, repo Repo) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Path, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ParseRemote(raw string) (model.RemoteIdentity, error) {
	provider, org, repo, err := parseRemoteParts(raw)
	if err != nil {
		return model.RemoteIdentity{}, err
	}
	repo = strings.TrimSuffix(repo, ".git")
	if strings.Contains(provider, "github") {
		provider = "github"
	} else if strings.Contains(provider, "gitlab") {
		provider = "gitlab"
	} else if strings.Contains(provider, "bitbucket") {
		provider = "bitbucket"
	} else {
		// Fallback to removing common TLDs if unknown
		provider = strings.Split(provider, ".")[0]
	}

	identity := model.RemoteIdentity{
		Provider: provider,
		Org:      org,
		Repo:     repo,
	}
	identity.OrgHash = crypto.SHA256(provider + ":" + org)
	identity.RepoHash = crypto.SHA256(provider + ":" + org + "/" + repo)
	return identity, nil
}

func parseRemoteParts(raw string) (string, string, string, error) {
	if strings.HasPrefix(raw, "git@") {
		re := regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+)$`)
		matches := re.FindStringSubmatch(raw)
		if len(matches) == 4 {
			return matches[1], matches[2], matches[3], nil
		}
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		cleaned := strings.TrimPrefix(path.Clean(parsed.Path), "/")
		parts := strings.Split(cleaned, "/")
		if len(parts) >= 2 {
			return parsed.Host, parts[0], parts[1], nil
		}
	}
	return "", "", "", fmt.Errorf("unsupported git remote URL")
}
