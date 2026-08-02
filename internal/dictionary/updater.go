package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// UpdateResult reports the outcome of Update.
type UpdateResult struct {
	// Updated is true only when a newer dictionary was downloaded, validated,
	// and atomically installed.
	Updated bool
	// Version is the resulting local dictionary version — the newly-applied
	// one if Updated, otherwise the version that was already local.
	Version string
}

// Update checks for a newer dictionary and, if available, downloads,
// validates, and atomically installs it (replacing ~/.proofboard/dictionary.json).
// This is the same atomic download+validate+rename flow previously duplicated
// between the `update-dictionary` command and the root command's startup
// check — now a single shared implementation.
//
// The dictionary is fetched directly from the backend's GET /cli/dictionary
// endpoint (dictionaryURL, already APIBaseURL+DictionaryPath at both call
// sites) in a single request — the endpoint is @Public()/@SkipCliAuth(), so
// no device/CLI auth token is needed, and it returns the full category
// signal data (keywords/paths/impact) plus the feature-keyword vocabulary
// directly, not just a version/URL pointer to a separate CDN download. This
// makes the categorization and feature-keyword vocabulary update live from
// the backend without a CLI release, reusing this same periodic check.
//
// Callers are responsible for their own throttling (see
// commands/root.go's 6h gate via state.LastDictionaryUpdateCheck) — Update
// itself always performs one real fetch when called; it does not rate-limit
// on its own.
func Update(ctx context.Context, homeDir, dictionaryURL string, local Dictionary) (UpdateResult, error) {
	remote, err := fetchDictionary(ctx, dictionaryURL)
	if err != nil {
		return UpdateResult{}, err
	}

	comparison, err := CompareVersions(remote.Version, local.Version)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("compare dictionary versions: %w", err)
	}
	if remote.Version == "" || comparison <= 0 {
		return UpdateResult{Updated: false, Version: local.Version}, nil
	}
	if err := Validate(remote); err != nil {
		return UpdateResult{}, fmt.Errorf("validate downloaded dictionary: %w", err)
	}

	dir := filepath.Join(homeDir, ".proofboard")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return UpdateResult{}, fmt.Errorf("create directory: %w", err)
	}

	tempPath := filepath.Join(dir, "dictionary.json.tmp")
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("create temp dictionary file: %w", err)
	}
	if err := json.NewEncoder(tempFile).Encode(remote); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return UpdateResult{}, fmt.Errorf("write temp dictionary file: %w", err)
	}
	tempFile.Close()

	targetPath := filepath.Join(dir, "dictionary.json")
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return UpdateResult{}, fmt.Errorf("replace dictionary file: %w", err)
	}

	return UpdateResult{Updated: true, Version: remote.Version}, nil
}

// fetchDictionary performs the single GET against the backend's public
// dictionary endpoint and decodes the response body directly as a
// Dictionary — the endpoint returns { version, categories, featureKeywords,
// updatedAt } verbatim, with no auth header and no success/data envelope
// (unlike the rest of the API, this route intentionally returns the raw
// payload — see cli-repos.controller.ts's getDictionary()).
func fetchDictionary(ctx context.Context, dictionaryURL string) (Dictionary, error) {
	parsed, err := url.Parse(dictionaryURL)
	if err != nil {
		return Dictionary{}, fmt.Errorf("parse dictionary URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return Dictionary{}, fmt.Errorf("dictionary endpoint must use HTTPS")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dictionaryURL, nil)
	if err != nil {
		return Dictionary{}, fmt.Errorf("create dictionary request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Dictionary{}, fmt.Errorf("fetch dictionary: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Dictionary{}, fmt.Errorf("dictionary endpoint returned %s", res.Status)
	}

	var remote Dictionary
	if err := json.NewDecoder(res.Body).Decode(&remote); err != nil {
		return Dictionary{}, fmt.Errorf("decode dictionary response: %w", err)
	}
	return remote, nil
}

// CompareVersions returns -1/0/1 as left is less than/equal to/greater than
// right, comparing dot-separated numeric version segments (an optional
// leading "v" is ignored). Moved here from internal/commands so both the
// `update-dictionary` command and Update (used by the root command's startup
// check) share one implementation.
func CompareVersions(left, right string) (int, error) {
	parse := func(value string) ([]int, error) {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		if value == "" {
			return nil, nil
		}
		parts := strings.Split(value, ".")
		numbers := make([]int, len(parts))
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil || number < 0 {
				return nil, fmt.Errorf("invalid version %q", value)
			}
			numbers[index] = number
		}
		return numbers, nil
	}
	leftParts, err := parse(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parse(right)
	if err != nil {
		return 0, err
	}
	width := len(leftParts)
	if len(rightParts) > width {
		width = len(rightParts)
	}
	for index := 0; index < width; index++ {
		var leftNumber, rightNumber int
		if index < len(leftParts) {
			leftNumber = leftParts[index]
		}
		if index < len(rightParts) {
			rightNumber = rightParts[index]
		}
		if leftNumber < rightNumber {
			return -1, nil
		}
		if leftNumber > rightNumber {
			return 1, nil
		}
	}
	return 0, nil
}
