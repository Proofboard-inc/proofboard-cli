package phase4

import (
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

func commitsWithKeywords(keywords ...string) []model.CommitSignal {
	commits := make([]model.CommitSignal, len(keywords))
	for i, kw := range keywords {
		commits[i] = model.CommitSignal{FeatureKeyword: kw}
	}
	return commits
}

// Regression test for the "comments feature" nonsense-milestone bug: a
// cluster where only one commit out of many happens to match a keyword must
// not let that single coincidental match name the whole milestone.
func TestDominantFeatureKeywordRequiresAMinimumShare(t *testing.T) {
	commits := commitsWithKeywords(
		"comments", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "",
	) // 45 commits, only 1 mentions "comments" — a ~2% share

	got := dominantFeatureKeyword(commits)

	if got != "" {
		t.Fatalf("dominantFeatureKeyword() = %q, want empty (single-commit match must not win a 45-commit cluster)", got)
	}
}

func TestDominantFeatureKeywordAcceptsAStrongMajority(t *testing.T) {
	commits := commitsWithKeywords("dashboard", "dashboard", "dashboard", "", "")

	got := dominantFeatureKeyword(commits)

	if got != "dashboard" {
		t.Fatalf("dominantFeatureKeyword() = %q, want %q (3/5 = 60%% clears the threshold)", got, "dashboard")
	}
}

func TestDominantFeatureKeywordEmptyWhenNoCommitsMatch(t *testing.T) {
	commits := commitsWithKeywords("", "", "")

	if got := dominantFeatureKeyword(commits); got != "" {
		t.Fatalf("dominantFeatureKeyword() = %q, want empty", got)
	}
}

func TestDominantFeatureKeywordHandlesEmptyCluster(t *testing.T) {
	if got := dominantFeatureKeyword(nil); got != "" {
		t.Fatalf("dominantFeatureKeyword(nil) = %q, want empty", got)
	}
}

// Regression test for a real dogfooding case: a 23-commit cluster where 5
// commits (21.7%) independently matched "checkout" — genuinely the
// cluster's theme, not a coincidence, but just under the old 0.4 (and then
// 0.25) share floor, so the milestone title fell back to a fully generic
// "medium-scale API effort" sentence. minFeatureKeywordShare was lowered to
// 0.2 specifically so 5 independent matches like this clear the bar; the
// separate minFeatureKeywordCount floor is what still rejects a lone
// coincidental match at a similar share (see the test below).
func TestDominantFeatureKeywordAcceptsRealMinoritySignal(t *testing.T) {
	commits := commitsWithKeywords(
		"checkout", "checkout", "checkout", "checkout", "checkout",
		"cart", "cart", "cart",
		"orders", "orders", "orders",
		"", "", "", "", "", "", "", "", "", "", "", "",
	) // 23 commits: checkout=5 (21.7%), cart=3, orders=3

	got := dominantFeatureKeyword(commits)

	if got != "checkout" {
		t.Fatalf("dominantFeatureKeyword() = %q, want %q (5/23 = 21.7%% is a real signal, not a coincidence)", got, "checkout")
	}
}

// A share just above 0.2 achieved by a single lucky match in a small cluster
// must still lose to the absolute-count floor — minFeatureKeywordShare
// alone isn't enough protection once it's this low.
func TestDominantFeatureKeywordSingleMatchStillRejectedAtLowShare(t *testing.T) {
	commits := commitsWithKeywords("dashboard", "", "", "", "") // 1/5 = 20%, exactly at the share floor but only 1 commit

	got := dominantFeatureKeyword(commits)

	if got != "" {
		t.Fatalf("dominantFeatureKeyword() = %q, want empty (a single match must not name a milestone even if its share clears the floor)", got)
	}
}
