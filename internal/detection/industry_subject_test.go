package detection

import (
	"reflect"
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

// IndustryHintsFromCommits fills in industries manifest-based detection
// (DetectStack/topIndustries, matched against dependency names) couldn't see
// — e.g. a repo that talks to an industry-specific API over raw HTTP with no
// matching npm dependency. These tests exercise it directly against raw
// commits, independent of any git repo fixture.

func TestIndustryHintsFromCommitsResolvesMatchingLabel(t *testing.T) {
	dict := model.Dictionary{
		IndustrySubjectKeywords: map[string][]string{
			"Logistics": {"shipday", "delivery route", "fleet tracking"},
			"Fintech":   {"ledger", "invoice"},
		},
	}
	raw := []model.RawCommit{
		{SHA: "a", Subject: []byte("integrate shipday webhook")},
		{SHA: "b", Subject: []byte("fix delivery route calculation")},
		{SHA: "c", Subject: []byte("unrelated cleanup")},
	}

	got := IndustryHintsFromCommits(raw, dict)
	if !reflect.DeepEqual(got, []string{"Logistics"}) {
		t.Fatalf("expected [Logistics], got %v", got)
	}
}

func TestIndustryHintsFromCommitsNoMatchReturnsEmpty(t *testing.T) {
	dict := model.Dictionary{
		IndustrySubjectKeywords: map[string][]string{
			"Logistics": {"shipday", "delivery route"},
		},
	}
	raw := []model.RawCommit{
		{SHA: "a", Subject: []byte("refactor auth middleware")},
		{SHA: "b", Subject: []byte("bump dependency versions")},
	}

	if got := IndustryHintsFromCommits(raw, dict); len(got) != 0 {
		t.Fatalf("expected empty hints, got %v", got)
	}
}

func TestIndustryHintsFromCommitsEmptyDictionaryReturnsEmpty(t *testing.T) {
	raw := []model.RawCommit{
		{SHA: "a", Subject: []byte("integrate shipday webhook")},
	}

	if got := IndustryHintsFromCommits(raw, model.Dictionary{}); len(got) != 0 {
		t.Fatalf("expected empty hints for nil IndustrySubjectKeywords, got %v", got)
	}
}

func TestIndustryHintsFromCommitsRanksByFrequencyThenAlphabeticalTie(t *testing.T) {
	dict := model.Dictionary{
		IndustrySubjectKeywords: map[string][]string{
			"Logistics":  {"delivery"},
			"Fintech":    {"invoice"},
			"Telecom":    {"sms"},
			"E-commerce": {"checkout"},
		},
	}
	// Each label needs at least 2 matches to clear the confidence floor
	// (minIndustryMatches) before it's reported at all. Fintech matches
	// three times, the rest twice each -> Fintech ranks first by frequency;
	// the remaining three-way tie at count 2 breaks alphabetically
	// (E-commerce, Logistics, Telecom) — all 4 fit under the cap of 5, so
	// none are dropped.
	raw := []model.RawCommit{
		{SHA: "a1", Subject: []byte("add delivery estimate")},
		{SHA: "a2", Subject: []byte("fix delivery tracking bug")},
		{SHA: "b1", Subject: []byte("send invoice reminder")},
		{SHA: "b2", Subject: []byte("reconcile invoice ledger")},
		{SHA: "b3", Subject: []byte("void duplicate invoice")},
		{SHA: "c1", Subject: []byte("send sms notification")},
		{SHA: "c2", Subject: []byte("retry failed sms carrier handoff")},
		{SHA: "d1", Subject: []byte("fix checkout bug")},
		{SHA: "d2", Subject: []byte("add checkout discount code")},
	}

	got := IndustryHintsFromCommits(raw, dict)
	want := []string{"Fintech", "E-commerce", "Logistics", "Telecom"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestIndustryHintsFromCommitsIsCaseInsensitive(t *testing.T) {
	dict := model.Dictionary{
		IndustrySubjectKeywords: map[string][]string{
			"Logistics": {"ShipDay", "Delivery Route"},
		},
	}
	// Two matches (mixed-case phrase, then upper-case) needed to clear the
	// confidence floor — this test's point is that BOTH still match
	// case-insensitively despite differing from the dictionary's casing and
	// from each other.
	raw := []model.RawCommit{
		{SHA: "a", Subject: []byte("Integrate SHIPDAY Webhook")},
		{SHA: "b", Subject: []byte("optimize delivery route calculation")},
	}

	got := IndustryHintsFromCommits(raw, dict)
	if !reflect.DeepEqual(got, []string{"Logistics"}) {
		t.Fatalf("expected case-insensitive match to resolve [Logistics], got %v", got)
	}
}
