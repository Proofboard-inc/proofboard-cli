package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const githubAPIBaseURL = "https://api.github.com"

type GitHubReleaseAsset struct {
	Name   string `json:"name"`
	APIURL string `json:"url"`
}

type GitHubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

func LatestGitHubRelease(ctx context.Context, repository string) (GitHubRelease, error) {
	repository = strings.Trim(repository, "/")
	if repository == "" {
		return GitHubRelease{}, fmt.Errorf("GitHub repository is required")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBaseURL, repository)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("create GitHub latest release request: %w", err)
	}
	authorizeGitHubRequest(req, false)
	client := &http.Client{CheckRedirect: validateReleaseRedirect}
	res, err := client.Do(req)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("fetch GitHub latest release: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return GitHubRelease{}, fmt.Errorf("GitHub latest release returned %s", res.Status)
	}
	var release GitHubRelease
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		return GitHubRelease{}, fmt.Errorf("decode GitHub latest release: %w", err)
	}
	return release, nil
}

func (r GitHubRelease) AssetURL(name string) (string, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name && strings.TrimSpace(asset.APIURL) != "" {
			return asset.APIURL, true
		}
	}
	return "", false
}

func authorizeGitHubRequest(req *http.Request, binaryAsset bool) {
	if req == nil || !strings.EqualFold(req.URL.Hostname(), "api.github.com") {
		return
	}
	if binaryAsset {
		req.Header.Set("Accept", "application/octet-stream")
	} else {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	token := strings.TrimSpace(os.Getenv("GH_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
