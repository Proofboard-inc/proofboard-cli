package crypto

import (
	"bytes"
	"testing"
)

func TestZeroBytes(t *testing.T) {
	data := []byte("secret information")
	ZeroBytes(data)

	expected := make([]byte, len("secret information"))
	if !bytes.Equal(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}
}

func TestDropStrings(t *testing.T) {
	data := []string{"path1", "path2"}
	result := DropStrings(data)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
	if data[0] != "" || data[1] != "" {
		t.Errorf("expected original slice to be empty strings, got %v", data)
	}
}
