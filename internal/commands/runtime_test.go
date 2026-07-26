package commands

import "testing"

func TestRequiresProviderVerificationOnlyForProofboardBackends(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"https://api-dev.proofboard.io":       true,
		"https://api.proofboard.io":           true,
		"https://api-dev.proofboard.io/path":  true,
		"https://api.proofboard.io.evil.test": false,
		"http://api-dev.proofboard.io":        false,
		"http://127.0.0.1:8080":               false,
		"https://example.test":                false,
	}
	for endpoint, expected := range cases {
		endpoint := endpoint
		expected := expected
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			if actual := requiresProviderVerification(endpoint); actual != expected {
				t.Fatalf("requiresProviderVerification(%q) = %t, want %t", endpoint, actual, expected)
			}
		})
	}
}
