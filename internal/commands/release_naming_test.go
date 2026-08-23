package commands

import "testing"

// Release assets carry the executable under two names. The product name
// matches every installer on the release page; the lowercase name is what
// versions up to 1.13.2 build to find their own update, and removing it would
// leave those installs unable to update at all.
func TestReleaseBinaryNamesCoverBothConventions(t *testing.T) {
	cases := []struct{ goos, goarch, suffix, want, legacy string }{
		{"darwin", "arm64", "", "Proofboard-Career-Agent-darwin-arm64", "proofboard-darwin-arm64"},
		{"linux", "amd64", "", "Proofboard-Career-Agent-linux-amd64", "proofboard-linux-amd64"},
		{"windows", "amd64", ".exe", "Proofboard-Career-Agent-windows-amd64.exe", "proofboard-windows-amd64.exe"},
	}
	for _, c := range cases {
		if got := releaseBinaryName(c.goos, c.goarch, c.suffix); got != c.want {
			t.Errorf("releaseBinaryName(%s,%s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
		if got := legacyReleaseBinaryName(c.goos, c.goarch, c.suffix); got != c.legacy {
			t.Errorf("legacyReleaseBinaryName(%s,%s) = %q, want %q — older installs build this exact string", c.goos, c.goarch, got, c.legacy)
		}
	}
}
