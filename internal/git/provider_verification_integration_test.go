//go:build integration

package git

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestRealPublicGitHubContributionSignals is deliberately opt-in because it
// uses the authenticated GitHub CLI and the live GitHub API. It never links or
// syncs the repository and therefore cannot alter Proofboard career data.
func TestRealPublicGitHubContributionSignals(t *testing.T) {
	repository := strings.TrimSpace(os.Getenv("PROOFBOARD_TEST_GITHUB_REPOSITORY"))
	if repository == "" {
		t.Skip("set PROOFBOARD_TEST_GITHUB_REPOSITORY=owner/repository")
	}
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		t.Fatalf("PROOFBOARD_TEST_GITHUB_REPOSITORY must be owner/repository")
	}

	var viewer struct {
		Login string `json:"login"`
	}
	if err := ghAPIJSON(context.Background(), []string{"user"}, &viewer); err != nil {
		t.Fatalf("read authenticated GitHub account: %v", err)
	}
	identity, err := ParseRemote("https://github.com/" + repository + ".git")
	if err != nil {
		t.Fatalf("parse live repository identity: %v", err)
	}
	verified, err := VerifyProviderContribution(context.Background(), identity, viewer.Login)
	if err != nil {
		t.Fatalf("collect live public GitHub signals: %v", err)
	}
	if verified.Private {
		t.Fatalf("%s is private; choose a public repository", repository)
	}
	if len(verified.CommitSHAs) == 0 {
		t.Fatalf("live GitHub response contained no commits attributed to the authenticated account")
	}
}
