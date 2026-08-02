package crypto

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CanonicalJSON produces deterministic JSON with object keys sorted
// lexicographically at every level. UseNumber preserves integer and decimal
// spellings while the value is normalized through interface maps.
//
// This MUST byte-for-byte match the backend's canonicalizePayloadForSigning
// (cli-payload-canonical.helper.ts), which uses Node's JSON.stringify — Node
// never escapes '&', '<', '>' in string values. Go's json.Marshal, by
// contrast, HTML-escapes those characters by default (e.g. "&" becomes
// "&"), which is only configurable via json.Encoder.SetEscapeHTML(false)
// — the package-level json.Marshal function used below has no such option.
// Any category name containing "&" (most of the dictionary's 25 categories
// do — "Authentication & Security", "API & Backend Services", etc.) produced
// a different signed byte sequence than what the backend verified against,
// making every device signature fail. SetEscapeHTML(false) here is what
// keeps the two sides byte-identical.
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
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(normalized); err != nil {
		return nil, fmt.Errorf("marshal canonical JSON: %w", err)
	}
	// json.Encoder.Encode appends a trailing newline that json.Marshal (and
	// Node's JSON.stringify) does not produce — strip it so the signed bytes
	// match exactly.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
