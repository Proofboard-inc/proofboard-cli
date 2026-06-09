package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ReleaseClient struct {
	baseURL    string
	httpClient *http.Client
}

type LatestVersion struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

func NewReleaseClient(baseURL string) ReleaseClient {
	return ReleaseClient{baseURL: baseURL, httpClient: &http.Client{}}
}

func (c ReleaseClient) Latest(ctx context.Context, route string) (LatestVersion, error) {
	endpoint, err := c.endpoint(route)
	if err != nil {
		return LatestVersion{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LatestVersion{}, fmt.Errorf("create latest request: %w", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return LatestVersion{}, fmt.Errorf("fetch latest version: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return LatestVersion{}, fmt.Errorf("release server returned %s", res.Status)
	}
	var latest LatestVersion
	if err := json.NewDecoder(res.Body).Decode(&latest); err != nil {
		return LatestVersion{}, fmt.Errorf("decode latest version: %w", err)
	}
	return latest, nil
}

func (c ReleaseClient) endpoint(route string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse release base URL: %w", err)
	}
	if base.Scheme != "https" && base.Hostname() != "127.0.0.1" && base.Hostname() != "localhost" {
		return "", fmt.Errorf("release endpoint must use HTTPS")
	}
	ref, err := url.Parse(route)
	if err != nil {
		return "", fmt.Errorf("parse release route: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}
