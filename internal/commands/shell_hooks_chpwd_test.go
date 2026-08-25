package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The directory-change hook is what makes project detection fire when someone
// cd's into a repository in a terminal that is already open. The startup line
// runs once per session, so without this hook the feature only ever works in a
// brand-new terminal — which looks, to a user, exactly like detection having
// silently stopped working.
//
// Nothing asserted that this hook is installed. The existing coverage tests
// the startup detect and notice lines, and the legacy migration, but a change
// that dropped the chpwd hook from the install would have left every one of
// those tests green.
func TestEnsureShellDetectionHooksInstallsDirectoryChangeHook(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shell string
		// Markers that must appear somewhere under the home directory. The
		// mechanism differs per shell, so each is checked for the thing that
		// actually makes a directory change trigger the CLI.
		markers []string
	}{
		{"bash", "/bin/bash", []string{"_proofboard_chpwd", "PROMPT_COMMAND"}},
		{"zsh", "/bin/zsh", []string{"_proofboard_chpwd", "add-zsh-hook chpwd"}},
		{"fish", "/usr/bin/fish", []string{"_proofboard_chpwd", "--on-variable PWD"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			setTestHome(t, homeDir)
			t.Setenv("SHELL", tc.shell)

			if _, _, err := ensureShellDetectionHooks(context.Background()); err != nil {
				t.Fatalf("ensureShellDetectionHooks: %v", err)
			}

			// Scan everything written under the home directory rather than
			// naming one rc file: which file a shell is configured through is
			// an implementation detail of the installer, and this test is
			// about the hook existing, not about where it lives.
			var written strings.Builder
			err := filepath.Walk(homeDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				written.Write(data)
				written.WriteString("\n")
				return nil
			})
			if err != nil {
				t.Fatalf("walk home: %v", err)
			}

			content := written.String()
			for _, marker := range tc.markers {
				if !strings.Contains(content, marker) {
					t.Errorf("no directory-change hook installed for %s: missing %q.\n"+
						"Detection would then only fire in a newly opened terminal, "+
						"never on cd within an existing one.", tc.name, marker)
				}
			}
		})
	}
}
