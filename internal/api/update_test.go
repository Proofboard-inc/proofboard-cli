package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseClientLatestResolvesBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest/download/latest.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"version":"1.8.11","url":"https://github.com/Proofboard-inc/proofboard-cli/releases/download/v1.8.11"}`))
	}))
	t.Cleanup(server.Close)

	client := NewReleaseClient(server.URL + "/releases/latest/download/")
	latest, err := client.Latest(context.Background(), "latest.json")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest.Version != "1.8.11" || !strings.HasSuffix(latest.URL, "/v1.8.11") {
		t.Fatalf("latest = %#v", latest)
	}
}

func TestReleaseClientRejectsInsecureExternalDownload(t *testing.T) {
	client := NewReleaseClient("https://github.com/Proofboard-inc/proofboard-cli/releases/latest/download/")
	err := client.Download(context.Background(), "http://example.com/proofboard", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("Download error = %v", err)
	}
}

func TestReleaseClientAllowsLocalHTTPDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("release"))
	}))
	t.Cleanup(server.Close)

	client := NewReleaseClient(server.URL)
	var output bytes.Buffer
	if err := client.Download(context.Background(), server.URL+"/proofboard", &output); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if output.String() != "release" {
		t.Fatalf("download = %q", output.String())
	}
}

func TestReleaseClientRejectsInsecureRedirect(t *testing.T) {
	client := NewReleaseClient("https://proofboard.io")
	redirectRequest, err := http.NewRequest(http.MethodGet, "http://example.com/proofboard", nil)
	if err != nil {
		t.Fatalf("create redirect request: %v", err)
	}
	err = client.httpClient.CheckRedirect(redirectRequest, nil)
	if err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("redirect error = %v", err)
	}
}
