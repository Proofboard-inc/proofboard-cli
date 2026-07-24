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
	refreshPath               string
}

func NewClient(baseURL string, linkPath string, checkPath string, syncPath string, optionalPaths ...string) Client {
	deviceKeyPath := ""
	refreshPath := ""
	if len(optionalPaths) > 0 {
		deviceKeyPath = optionalPaths[0]
	}
	if len(optionalPaths) > 1 {
		refreshPath = optionalPaths[1]
	}
	return Client{
		baseURL:                   baseURL,
		linkPath:                  linkPath,
		checkPath:                 checkPath,
		syncPath:                  syncPath,
		deviceKeyRegistrationPath: deviceKeyPath,
		refreshPath:               refreshPath,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func NewClientWithHTTP(baseURL string, linkPath string, checkPath string, syncPath string, httpClient *http.Client, optionalPaths ...string) Client {
	client := NewClient(baseURL, linkPath, checkPath, syncPath, optionalPaths...)
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
				if tight, marshalErr := json.Marshal(request); marshalErr == nil {
					reqStr = redactJSONForLog(tight)
				}
			}
			redactedEndpoint := "[REDACTED]"
			timestamp := time.Now().UTC().Format(time.RFC3339)
			respStr := strings.TrimSpace(redactJSONForLog(bodyBytes))
			respStr = strings.ReplaceAll(respStr, "\n", " ")
			logText := fmt.Sprintf("%s — HTTP %s — %s — REQUEST %s — RESPONSE (%s) %s\n", timestamp, method, redactedEndpoint, reqStr, res.Status, respStr)
			file.Write([]byte(logText))
			file.Close()
		}
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("API returned %s: %s", res.Status, redactJSONForLog(bodyBytes))
	}
	if response == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func redactJSONForLog(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "[NON_JSON_RESPONSE_REDACTED]"
	}
	redactSensitiveJSON(value)
	redacted, err := json.Marshal(value)
	if err != nil {
		return "[REDACTED]"
	}
	return string(redacted)
}

func redactSensitiveJSON(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(key)
			if strings.Contains(normalized, "token") || strings.Contains(normalized, "signature") || normalized == "orghash" || normalized == "repohash" || normalized == "emailhash" {
				typed[key] = "[REDACTED]"
				continue
			}
			redactSensitiveJSON(child)
		}
	case []any:
		for _, child := range typed {
			redactSensitiveJSON(child)
		}
	}
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
