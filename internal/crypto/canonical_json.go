package crypto

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CanonicalJSON produces deterministic JSON with object keys sorted
// lexicographically at every level. UseNumber preserves integer and decimal
// spellings while the value is normalized through interface maps.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical JSON input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("decode canonical JSON input: %w", err)
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical JSON: %w", err)
	}
	return canonical, nil
}
