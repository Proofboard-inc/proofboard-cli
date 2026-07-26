package git

import (
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func TestApplyPublicGitHubSignalsIsAdditive(t *testing.T) {
	t.Parallel()
	commits := []model.RawCommit{{SHA: "own-a"}, {SHA: "own-b"}}

	privateResult, applied := ApplyPublicGitHubSignals(commits, ProviderVerification{
		Private:    true,
		CommitSHAs: map[string]struct{}{"own-a": {}},
	})
	if applied || len(privateResult) != len(commits) {
		t.Fatalf("private repository signals changed local commits: %#v", privateResult)
	}

	unavailableResult, applied := ApplyPublicGitHubSignals(commits, ProviderVerification{})
	if applied || len(unavailableResult) != len(commits) {
		t.Fatalf("unavailable signals changed local commits: %#v", unavailableResult)
	}

	nonMatchingResult, applied := ApplyPublicGitHubSignals(commits, ProviderVerification{
		CommitSHAs: map[string]struct{}{"someone-else": {}},
	})
	if applied || len(nonMatchingResult) != len(commits) {
		t.Fatalf("non-matching signals changed local commits: %#v", nonMatchingResult)
	}

	publicResult, applied := ApplyPublicGitHubSignals(commits, ProviderVerification{
		CommitSHAs: map[string]struct{}{"own-b": {}},
	})
	if !applied || len(publicResult) != 1 || publicResult[0].SHA != "own-b" {
		t.Fatalf("public contribution signals were not applied: %#v, applied=%v", publicResult, applied)
	}
}
