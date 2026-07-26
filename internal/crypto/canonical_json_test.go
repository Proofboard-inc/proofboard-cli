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
