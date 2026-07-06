package git

import "testing"

func TestParseRemoteSupportsSSHAndHTTPS(t *testing.T) {
	t.Parallel()
	tests := []string{
		"git@github.com:Proofboard-inc/proofboard-cli.git",
		"https://github.com/Proofboard-inc/proofboard-cli.git",
	}
	for _, raw := range tests {
		identity, err := ParseRemote(raw)
		if err != nil {
			t.Fatalf("ParseRemote(%q) returned error: %v", raw, err)
		}
		if identity.Provider != "github" || identity.Org != "Proofboard-inc" || identity.Repo != "proofboard-cli" {
			t.Fatalf("unexpected identity for %q: %#v", raw, identity)
		}
		if identity.OrgHash == "" || identity.RepoHash == "" || identity.OrgHash == identity.RepoHash {
			t.Fatalf("expected distinct hashes for %q", raw)
		}
	}
}
