package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	baseURL                   string
	httpClient                *http.Client
	linkPath                  string
	checkPath                 string
	syncPath                  string
	deviceKeyRegistrationPath string
}

func NewClient(baseURL string, linkPath string, checkPath string, syncPath string, deviceKeyRegistrationPath ...string) Client {
	path := ""
	if len(deviceKeyRegistrationPath) > 0 {
		path = deviceKeyRegistrationPath[0]
	}
	return Client{
		baseURL:                   baseURL,
		linkPath:                  linkPath,
		checkPath:                 checkPath,
		syncPath:                  syncPath,
		deviceKeyRegistrationPath: path,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func NewClientWithHTTP(baseURL string, linkPath string, checkPath string, syncPath string, httpClient *http.Client, deviceKeyRegistrationPath ...string) Client {
	client := NewClient(baseURL, linkPath, checkPath, syncPath, deviceKeyRegistrationPath...)
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
		tightData, _ := json.Marshal(request)
		body = bytes.NewReader(tightData)
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

	// Write debug log to sync.log
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		logPath := homeDir + "/.proofboard/sync.log"
		if file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			reqStr := "(none)"
			if request != nil {
				// We can marshal it to a map to redact hashes
				var m map[string]any
				if tight, err := json.Marshal(request); err == nil {
					if json.Unmarshal(tight, &m) == nil {
						if _, ok := m["orgHash"]; ok {
							m["orgHash"] = "[REDACTED]"
						}
						if _, ok := m["repoHash"]; ok {
							m["repoHash"] = "[REDACTED]"
						}
						if _, ok := m["emailHash"]; ok {
							m["emailHash"] = "[REDACTED]"
						}
						if _, ok := m["deviceSignature"]; ok {
							m["deviceSignature"] = "[REDACTED]"
						}
						redacted, _ := json.Marshal(m)
						reqStr = string(redacted)
					} else {
						reqStr = string(tight)
					}
				}
			}
			redactedEndpoint := "[REDACTED]"
			timestamp := time.Now().UTC().Format(time.RFC3339)
			respStr := strings.TrimSpace(string(bodyBytes))
			respStr = strings.ReplaceAll(respStr, "\n", " ")
			logText := fmt.Sprintf("%s — HTTP %s — %s — REQUEST %s — RESPONSE (%s) %s\n", timestamp, method, redactedEndpoint, reqStr, res.Status, respStr)
			file.Write([]byte(logText))
			file.Close()
		}
	}

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
