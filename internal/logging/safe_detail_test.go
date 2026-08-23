package logging

import (
	"strings"
	"testing"
)

// The log must keep enough to diagnose a failure. Reducing every error to
// "[details redacted]" left a developer whose sync returned 400 with no way
// to learn anything about why, which protects nothing: a status code is not a
// token, an address, or a repository name.
func TestSafeLogDetailKeepsDiagnosableFailures(t *testing.T) {
	cases := []struct{ in, want string }{
		{"transmit sync payload: API returned 400", "api returned 400"},
		{"download returned status 404 Not Found", "returned status 404"},
		{"post https://api.proofboard.io/api/v1/cli/sync: context deadline exceeded", "context deadline exceeded"},
		{`dial tcp: lookup api.proofboard.io: no such host`, "no such host"},
		{"register device signing key: API returned 401", "api returned 401"},
	}
	for _, c := range cases {
		got := safeLogDetail(c.in)
		if !strings.Contains(got, c.want) {
			t.Errorf("safeLogDetail(%q) = %q, want it to keep %q", c.in, got, c.want)
		}
	}
}

// Whatever is kept is assembled from the recognised fragments only, so
// identifying material in the original message cannot survive.
func TestSafeLogDetailDropsIdentifyingMaterial(t *testing.T) {
	secrets := []string{
		"api.proofboard.io", "danroyal001", "@gmail.com", "igamer-app-js",
		"Bearer", "eyJhbGciOi", "/Users/DELL", "Proofboard-inc",
	}
	inputs := []string{
		"post https://api.proofboard.io/api/v1/cli/sync: API returned 400",
		"sync repo igamer-app-js for danroyal001@gmail.com: returned status 403",
		"auth failed for token eyJhbGciOi.abc.def: API returned 401",
		`open C:\Users\DELL\.proofboard\credentials.json: no such host`,
	}
	for _, in := range inputs {
		got := safeLogDetail(in)
		for _, s := range secrets {
			if strings.Contains(strings.ToLower(got), strings.ToLower(s)) {
				t.Errorf("safeLogDetail(%q) leaked %q in %q", in, s, got)
			}
		}
	}
}

// Anything with no recognised failure information is still fully redacted,
// rather than being passed through on the assumption it looks harmless.
func TestSafeLogDetailRedactsUnrecognisedText(t *testing.T) {
	for _, in := range []string{
		"something went wrong in /home/user/private-repo",
		"failed for user@example.com",
	} {
		if got := safeLogDetail(in); got != redactedLogDetail {
			t.Errorf("safeLogDetail(%q) = %q, want it fully redacted", in, got)
		}
	}
}
