package dictionary

import "testing"

func TestCompareVersions(t *testing.T) {
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
			got, err := CompareVersions(test.left, test.right)
			if err != nil {
				t.Fatalf("CompareVersions() error: %v", err)
			}
			if got != test.comparison {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.comparison)
			}
		})
	}
}

func TestCompareVersionsRejectsInvalidVersion(t *testing.T) {
	if _, err := CompareVersions("1.9.next", "1.9.3"); err == nil {
		t.Fatal("CompareVersions() accepted a non-numeric version")
	}
}
