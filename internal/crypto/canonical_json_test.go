package crypto

import (
	"testing"
)

func TestCanonicalJSONSortsNestedKeysAndPreservesNumbers(t *testing.T) {
	input := struct {
		Z int            `json:"z"`
		A map[string]any `json:"a"`
	}{
		Z: 7,
		A: map[string]any{"z": 2, "a": 1.25},
	}
	got, err := CanonicalJSON(input)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	const want = `{"a":{"a":1.25,"z":2},"z":7}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
}

// FIX: Go's json.Marshal HTML-escapes '&', '<', '>' by default (e.g. "&"
// becomes "&"), but the backend's canonicalizePayloadForSigning uses
// Node's JSON.stringify, which never does. Since most of the CLI category
// dictionary's 25 entries contain "&" (e.g. "Authentication & Security"),
// every real category previously produced a different signed byte sequence
// than what the backend verified against, failing every device signature.
// Expected value below was cross-checked directly against Node:
// `node -e 'console.log(JSON.stringify({category: "Authentication & Security", num: 42, other: "a<b>c&d"}))'`
// → {"category":"Authentication & Security","num":42,"other":"a<b>c&d"}
func TestCanonicalJSONDoesNotHTMLEscape(t *testing.T) {
	input := map[string]any{
		"category": "Authentication & Security",
		"other":    "a<b>c&d",
		"num":      42,
	}
	got, err := CanonicalJSON(input)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	const want = `{"category":"Authentication & Security","num":42,"other":"a<b>c&d"}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s (matches Node's JSON.stringify byte-for-byte)", got, want)
	}
}
