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
