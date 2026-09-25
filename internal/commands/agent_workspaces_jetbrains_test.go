package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJetBrainsRecentProjectEntriesExtractsPaths(t *testing.T) {
	doc := []byte(`<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/Dev/proofboard">
          <value>
            <RecentProjectMetaInfo>
              <option name="projectOpenTimestamp" value="1234" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/Dev/other-project">
          <value>
            <RecentProjectMetaInfo />
          </value>
        </entry>
      </map>
    </option>
  </component>
</application>`)

	entries := jetBrainsRecentProjectEntries(doc)
	want := map[string]bool{
		"$USER_HOME$/Dev/proofboard":    true,
		"$USER_HOME$/Dev/other-project": true,
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %#v, want %d entries", entries, len(want))
	}
	for _, entry := range entries {
		if !want[entry] {
			t.Fatalf("unexpected entry %q", entry)
		}
	}
}

func TestDiscoverJetBrainsWorkspacesFindsRepositoryViaUserHomeSubstitution(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := t.TempDir()
	setTestHome(t, homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	roots := jetBrainsConfigRoots()
	if len(roots) == 0 {
		t.Fatal("jetBrainsConfigRoots() returned nothing")
	}
	productDir := filepath.Join(roots[0], "PyCharm2025.2", "options")
	if err := os.MkdirAll(productDir, 0o700); err != nil {
		t.Fatalf("mkdir jetbrains options dir: %v", err)
	}

	// $USER_HOME$ is JetBrains's own placeholder, substituted at read time,
	// so the fixture must reference the repo relative to homeDir the same
	// way a real recentProjects.xml would.
	relPath, err := filepath.Rel(homeDir, repoDir)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}
	doc := `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/` + filepath.ToSlash(relPath) + `">
          <value><RecentProjectMetaInfo /></value>
        </entry>
      </map>
    </option>
  </component>
</application>`
	if err := os.WriteFile(filepath.Join(productDir, "recentProjects.xml"), []byte(doc), 0o600); err != nil {
		t.Fatalf("write recentProjects.xml: %v", err)
	}

	seen := make(map[string]bool)
	var workspaces []string
	discoverJetBrainsWorkspaces(context.Background(), seen, &workspaces)

	if len(workspaces) != 1 {
		t.Fatalf("workspaces = %#v, want exactly the repo", workspaces)
	}
	got, err := filepath.EvalSymlinks(workspaces[0])
	if err != nil {
		t.Fatalf("resolve workspace path: %v", err)
	}
	want, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("resolve repo path: %v", err)
	}
	if got != want {
		t.Fatalf("workspace = %q, want %q", got, want)
	}
}
