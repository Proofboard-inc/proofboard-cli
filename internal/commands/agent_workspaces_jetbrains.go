package commands

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// jetBrainsConfigRoots returns the per-OS parent directory under which every
// JetBrains product keeps its own "<Product><Version>" config directory
// (for example "PyCharm2025.2", "IntelliJIdea2025.2"). The directory naming
// includes the product version, so this globs rather than assuming one.
func jetBrainsConfigRoots() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{filepath.Join(homeDir, "Library", "Application Support", "JetBrains")}
	case "windows":
		return []string{filepath.Join(os.Getenv("APPDATA"), "JetBrains")}
	default:
		return []string{filepath.Join(homeDir, ".config", "JetBrains")}
	}
}

// jetBrainsRecentProjectsFiles globs every installed product/version
// directory for its recentProjects.xml.
func jetBrainsRecentProjectsFiles() []string {
	var files []string
	for _, root := range jetBrainsConfigRoots() {
		matches, err := filepath.Glob(filepath.Join(root, "*", "options", "recentProjects.xml"))
		if err != nil {
			continue
		}
		files = append(files, matches...)
	}
	return files
}

func discoverJetBrainsWorkspaces(ctx context.Context, seen map[string]bool, workspaces *[]string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, path := range jetBrainsRecentProjectsFiles() {
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 8*1024*1024 {
			continue
		}
		for _, candidate := range jetBrainsRecentProjectEntries(data) {
			candidate = strings.ReplaceAll(candidate, "$USER_HOME$", homeDir)
			addWorkspaceCandidate(ctx, candidate, seen, workspaces)
		}
	}
}

// jetBrainsRecentProjectEntries extracts every recent-project path from a
// recentProjects.xml document. Paths are stored as the "key" attribute of
// each <entry> under the RecentProjectsManager's "additionalInfo" map, e.g.
// <entry key="$USER_HOME$/Dev/proofboard"><value>...</value></entry>.
// $USER_HOME$ is JetBrains's own path-variable placeholder, substituted by
// the caller. This walks the document generically rather than assuming a
// fixed nesting depth, since that nesting has changed across IDE versions.
func jetBrainsRecentProjectEntries(data []byte) []string {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var entries []string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "entry" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "key" && strings.Contains(attr.Value, "/") {
				entries = append(entries, attr.Value)
			}
		}
	}
	return entries
}
