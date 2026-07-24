package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	return ReleaseClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			CheckRedirect: validateReleaseRedirect,
		},
	}
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

func (c ReleaseClient) Download(ctx context.Context, route string, w io.Writer) error {
	endpoint := route
	if !strings.HasPrefix(route, "http://") && !strings.HasPrefix(route, "https://") {
		var err error
		endpoint, err = c.endpoint(route)
		if err != nil {
			return err
		}
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse download URL: %w", err)
	}
	if err := validateReleaseURL(parsedEndpoint); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download returned status %s", res.Status)
	}
	_, err = io.Copy(w, res.Body)
	if err != nil {
		return fmt.Errorf("write download data: %w", err)
	}
	return nil
}

func validateReleaseRedirect(req *http.Request, _ []*http.Request) error {
	return validateReleaseURL(req.URL)
}

func validateReleaseURL(candidate *url.URL) error {
	if candidate.Scheme != "https" && candidate.Hostname() != "127.0.0.1" && candidate.Hostname() != "localhost" {
		return fmt.Errorf("release download must use HTTPS")
	}
	return nil
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
