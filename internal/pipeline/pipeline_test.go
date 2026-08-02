package pipeline

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/model"
)

func TestPipelinePayloadContainsNoProprietaryText(t *testing.T) {
	t.Parallel()
	dict, err := dictionary.LoadDefault(context.Background())
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	raw := []model.RawCommit{{
		SHA:          "abc",
		Timestamp:    time.Unix(1710000000, 0),
		Additions:    10,
		Deletions:    2,
		FilesChanged: 1,
		Subject:      []byte("secret Acme payment project"),
		FilePaths:    []string{"clients/acme/payments/secret.go"},
		AuthorEmail:  "dev@example.com",
		Repository:   "secret-repo",
		Organization: "secret-org",
	}}
	payload, err := New(dict).Run(context.Background(), RunInput{
		Raw:       raw,
		OrgHash:   "org-hash",
		RepoHash:  "repo-hash",
		EmailHash: "email-hash",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"secret", "Acme", "clients/", "dev@example.com", "secret-repo", "secret-org"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("payload leaked %q: %s", forbidden, text)
		}
	}
}

// RunInput.Stack must flow through unchanged into the final
// SyncPayload.Stack — plumbing only, no live filesystem detection needed.
func TestPipelinePassesStackThrough(t *testing.T) {
	t.Parallel()
	dict, err := dictionary.LoadDefault(context.Background())
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	stack := &model.StackReport{
		Languages: map[string]int{"Go": 3},
		TechStack: []string{"Gin"},
		HasCI:     true,
	}

	payload, err := New(dict).Run(context.Background(), RunInput{
		Raw:       nil,
		OrgHash:   "org-hash",
		RepoHash:  "repo-hash",
		EmailHash: "email-hash",
		Stack:     stack,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if payload.Stack != stack {
		t.Fatalf("expected payload.Stack to be the same StackReport passed in, got %#v", payload.Stack)
	}

	payloadNoStack, err := New(dict).Run(context.Background(), RunInput{
		Raw:       nil,
		OrgHash:   "org-hash",
		RepoHash:  "repo-hash",
		EmailHash: "email-hash",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if payloadNoStack.Stack != nil {
		t.Fatalf("expected nil Stack when RunInput.Stack is omitted, got %#v", payloadNoStack.Stack)
	}
}
