package commands

import "testing"

func TestCompareDictionaryVersions(t *testing.T) {
	tests := []struct {
		name       string
		left       string
		right      string
		comparison int
	}{
		{name: "newer patch", left: "1.9.3", right: "1.9.2", comparison: 1},
		{name: "older patch", left: "v1.9.2", right: "1.9.3", comparison: -1},
		{name: "same version", left: "v1.9.3", right: "1.9.3", comparison: 0},
		{name: "missing patch equals zero", left: "2.0", right: "2.0.0", comparison: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := compareDictionaryVersions(test.left, test.right)
			if err != nil {
				t.Fatalf("compareDictionaryVersions() error: %v", err)
			}
			if got != test.comparison {
				t.Fatalf("compareDictionaryVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.comparison)
			}
		})
	}
}

func TestCompareDictionaryVersionsRejectsInvalidVersion(t *testing.T) {
	if _, err := compareDictionaryVersions("1.9.next", "1.9.3"); err == nil {
		t.Fatal("compareDictionaryVersions() accepted a non-numeric version")
	}
}
