package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	linkPath   string
	syncPath   string
}

func NewClient(baseURL string, linkPath string, syncPath string) Client {
	return Client{
		baseURL:  baseURL,
		linkPath: linkPath,
		syncPath: syncPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewClientWithHTTP(baseURL string, linkPath string, syncPath string, httpClient *http.Client) Client {
	client := NewClient(baseURL, linkPath, syncPath)
	client.httpClient = httpClient
	return client
}

func (c Client) requestJSON(ctx context.Context, method string, path string, token string, query url.Values, request any, response any) error {
	endpoint, err := c.endpoint(path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("parse endpoint: %w", err)
		}
		q := u.Query()
		for k, v := range query {
			for _, val := range v {
				q.Add(k, val)
			}
		}
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}

	var body io.Reader
	if request != nil {
		data, err := json.MarshalIndent(request, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		fmt.Printf("\n--- HTTP %s %s ---\nREQUEST:\n%s\n", method, endpoint, string(data))
		tightData, _ := json.Marshal(request)
		body = bytes.NewReader(tightData)
	} else {
		fmt.Printf("\n--- HTTP %s %s ---\nREQUEST: (none)\n", method, endpoint)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if request != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()
	
	bodyBytes, _ := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	fmt.Printf("RESPONSE (%s):\n%s\n--------------------\n\n", res.Status, string(bodyBytes))
	
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("API returned %s: %s", res.Status, string(bodyBytes))
	}
	if response == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c Client) postJSON(ctx context.Context, path string, token string, request any, response any) error {
	return c.requestJSON(ctx, http.MethodPost, path, token, nil, request, response)
}

func (c Client) getJSON(ctx context.Context, path string, token string, query url.Values, response any) error {
	return c.requestJSON(ctx, http.MethodGet, path, token, query, nil, response)
}

func (c Client) patchJSON(ctx context.Context, path string, token string, request any, response any) error {
	return c.requestJSON(ctx, http.MethodPatch, path, token, nil, request, response)
}

func (c Client) endpoint(route string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse API base URL: %w", err)
	}
	if base.Scheme != "https" && base.Hostname() != "127.0.0.1" && base.Hostname() != "localhost" {
		return "", fmt.Errorf("API endpoint must use HTTPS")
	}
	ref, err := url.Parse(route)
	if err != nil {
		return "", fmt.Errorf("parse API route: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}
