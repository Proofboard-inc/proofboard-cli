package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

// A link request carrying a non-nil Stack must serialize a non-empty
// techStack (and structural flags) in the outgoing JSON body.
func TestLinkRequestIncludesNonEmptyTechStack(t *testing.T) {
	var captured LinkRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    LinkResponse{ProjectID: "project-1"},
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "/link", "", "")
	req := LinkRequest{
		OrgHash:  "org-hash",
		RepoHash: "repo-hash",
		Provider: "github",
		Stack: &model.StackReport{
			Languages: map[string]int{"Go": 4},
			TechStack: []string{"Gin"},
			HasCI:     true,
		},
	}
	if _, err := client.Link(context.Background(), "token", req); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if captured.Stack == nil {
		t.Fatal("expected request body to include a non-nil stack object")
	}
	if len(captured.Stack.TechStack) == 0 {
		t.Fatalf("expected non-empty techStack, got %#v", captured.Stack.TechStack)
	}
	if !captured.Stack.HasCI {
		t.Fatal("expected hasCI to be true")
	}
}

// A link request that omits Stack (old CLI behavior) must not send a stack
// field at all.
func TestLinkRequestOmitsStackWhenNil(t *testing.T) {
	var raw map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    LinkResponse{ProjectID: "project-1"},
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "/link", "", "")
	req := LinkRequest{OrgHash: "org-hash", RepoHash: "repo-hash", Provider: "github"}
	if _, err := client.Link(context.Background(), "token", req); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if _, ok := raw["stack"]; ok {
		t.Fatal("expected no stack field in request body when Stack is nil")
	}
}
