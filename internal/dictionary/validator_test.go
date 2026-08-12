package dictionary

import (
	"context"
	"testing"
)

func TestLoadDefaultDictionaryIsValid(t *testing.T) {
	t.Parallel()
	dict, err := LoadDefault(context.Background())
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if err := Validate(dict); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// Regression test for the "tech stack only detected Jest, NestJS" bug: the
// checked-in embedded fallback (internal/dictionary/dictionary.json, used by
// LoadDefault on a fresh install or whenever no live fetch has ever landed)
// had gone stale at 0 stackSignals/industrySignals — a NestJS+MongoDB+
// Stripe+Redis backend detected as just "Jest, NestJS" until the very first
// live dictionary fetch happened to succeed. Guards against the bundled
// fallback silently degrading back to that state.
func TestEmbeddedDictionaryHasStackAndIndustrySignals(t *testing.T) {
	file, err := embeddedDictionary.Open("dictionary.json")
	if err != nil {
		t.Fatalf("open embedded dictionary: %v", err)
	}
	defer file.Close()

	dict, err := Load(context.Background(), file)
	if err != nil {
		t.Fatalf("load embedded dictionary: %v", err)
	}
	if len(dict.StackSignals) == 0 {
		t.Error("embedded dictionary.json has no stackSignals — bundle a current export from GET /api/v1/cli/dictionary")
	}
	if len(dict.IndustrySignals) == 0 {
		t.Error("embedded dictionary.json has no industrySignals — bundle a current export from GET /api/v1/cli/dictionary")
	}
}
