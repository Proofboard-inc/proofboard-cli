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

func TestParseRemoteSupportsProvidersAndNestedGitLabGroups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw      string
		provider string
		org      string
		repo     string
	}{
		{
			raw:      "git@gitlab.com:engineering/platform/mobile/app.git",
			provider: "gitlab",
			org:      "engineering/platform/mobile",
			repo:     "app",
		},
		{
			raw:      "https://gitlab.com/engineering/platform/mobile/app.git",
			provider: "gitlab",
			org:      "engineering/platform/mobile",
			repo:     "app",
		},
		{
			raw:      "git@bitbucket.org:team/private-project.git",
			provider: "bitbucket",
			org:      "team",
			repo:     "private-project",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.raw, func(t *testing.T) {
			t.Parallel()
			identity, err := ParseRemote(test.raw)
			if err != nil {
				t.Fatalf("ParseRemote(%q): %v", test.raw, err)
			}
			if identity.Provider != test.provider || identity.Org != test.org || identity.Repo != test.repo {
				t.Fatalf("ParseRemote(%q) = %#v", test.raw, identity)
			}
		})
	}
}
