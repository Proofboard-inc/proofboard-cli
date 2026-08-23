package detection

import (
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

// A manifest dependency and a module path that point at the same industry are
// two independent signals and must accumulate. Counting only the path matches
// (assigning instead of adding) dropped the manifest evidence entirely, which
// pushed genuine hints below minIndustryMatches — the case where two signals
// agree is the strongest one, not one to discard.
func TestIndustrySignalsFromManifestAndPathAccumulate(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "package.json", `{"dependencies":{"stripe":"^14.0.0"}}`)
	writeFile(t, dir, "src/payments/charge.ts", "export const x = 1;\n")
	commitAll(t, dir)

	report, err := DetectStack(dir, model.Dictionary{
		IndustrySignals:      map[string]string{"stripe": "Fintech"},
		IndustryPathKeywords: map[string][]string{"Fintech": {"payments"}},
	})
	if err != nil {
		t.Fatalf("DetectStack: %v", err)
	}
	for _, h := range report.IndustryHints {
		if h == "Fintech" {
			return
		}
	}
	t.Fatalf("Fintech missing: manifest signal plus path directory should reach minIndustryMatches=%d, got %v",
		minIndustryMatches, report.IndustryHints)
}
