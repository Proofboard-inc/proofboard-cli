package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Error is the structured form of a non-2xx API response. Error deliberately
// omits Message from its string representation: callers can make decisions
// from the typed fields without accidentally writing server-provided details
// (which may contain proprietary project data) to logs.
type Error struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
}

// Error deliberately excludes e.Message. The server echoes request content
// back in it — the test fixture for this is a message naming a repository and
// an employer — so printing it would leak exactly the identifiers this tool
// exists to keep local. The status code and the structured code are safe and
// are what a failure is diagnosed from; the message stays on the struct for a
// caller that has a safe use for it.
func (e *Error) Error() string {
	if e == nil {
		return "API request failed"
	}
	if strings.TrimSpace(e.Code) != "" {
		return fmt.Sprintf("API returned %d (%s)", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("API returned %d", e.StatusCode)
}

func IsStatus(err error, statusCode int) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == statusCode
}

func parseAPIError(statusCode int, header http.Header, body []byte) *Error {
	apiErr := &Error{StatusCode: statusCode}
	// message arrives as a string from some endpoints and as an array of
	// validation failures from others. Decoding it only as a string meant an
	// array silently produced an empty message, which is why a rejected sync
	// reported no reason at all while the server had listed five.
	var response struct {
		Code    string          `json:"code"`
		Error   string          `json:"error"`
		Message json.RawMessage `json:"message"`
	}
	if json.Unmarshal(body, &response) == nil {
		apiErr.Code = strings.TrimSpace(response.Code)
		if apiErr.Code == "" {
			apiErr.Code = strings.TrimSpace(response.Error)
		}
		apiErr.Message = decodeAPIMessage(response.Message)
	}
	if retryAfter := strings.TrimSpace(header.Get("Retry-After")); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
			if seconds == 0 {
				apiErr.RetryAfter = time.Nanosecond
			}
		}
	}
	return apiErr
}

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

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return parseAPIError(res.StatusCode, res.Header, bodyBytes)
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
	safe := make(map[string]any)
	if object, ok := value.(map[string]any); ok {
		for _, key := range []string{"statusCode"} {
			switch item := object[key].(type) {
			case string, float64, bool:
				safe[key] = item
			}
		}
	}
	redacted, err := json.Marshal(safe)
	if err != nil {
		return "[REDACTED]"
	}
	return string(redacted)
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

func (c Client) deleteJSON(ctx context.Context, path string, token string, response any) error {
	return c.requestJSON(ctx, http.MethodDelete, path, token, nil, nil, response)
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

// decodeAPIMessage reads the server's message field, which is a string on some
// endpoints and an array of validation failures on others.
func decodeAPIMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return strings.TrimSpace(single)
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		parts := make([]string, 0, len(many))
		for _, m := range many {
			if m = strings.TrimSpace(m); m != "" {
				parts = append(parts, m)
			}
		}
		return strings.Join(parts, "; ")
	}
	return ""
}
