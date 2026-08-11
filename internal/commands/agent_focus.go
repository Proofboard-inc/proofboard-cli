package commands

import (
	"path/filepath"
	"strings"
)

// ideFrontmostAliases maps a configured IDE process short name (as used by
// discoverIDEWorkspaces/configuredIDENames) to the substrings that name shows
// up as in a frontmost/foreground GUI application name on macOS or Windows.
// Terminal-based editors (vim/nvim) have no distinct GUI frontmost app of
// their own — their host terminal is what's actually frontmost — so they are
// intentionally not matched by the focus check.
var ideFrontmostAliases = map[string][]string{
	"code":          {"code", "visual studio code"},
	"code-insiders": {"code - insiders", "visual studio code - insiders"},
	"cursor":        {"cursor"},
	"webstorm":      {"webstorm"},
	"idea":          {"idea", "intellij"},
	"zed":           {"zed"},
	"sublime_text":  {"sublime"},
}

// matchesFrontmostIDEName reports whether the frontmost GUI application name
// corresponds to one of the configured IDE process names.
func matchesFrontmostIDEName(frontApp string, ideNames []string) bool {
	frontApp = strings.ToLower(strings.TrimSpace(frontApp))
	frontApp = strings.TrimSuffix(frontApp, ".exe")
	if frontApp == "" {
		return false
	}
	for _, name := range ideNames {
		name = strings.ToLower(strings.TrimSpace(name))
		name = strings.TrimSuffix(name, ".exe")
		aliases, ok := ideFrontmostAliases[name]
		if !ok {
			aliases = []string{name}
		}
		for _, alias := range aliases {
			if strings.Contains(frontApp, alias) {
				return true
			}
		}
	}
	return false
}

// windowTitleMatchesWorkspace reports whether a focused window's title looks
// like it belongs to the given workspace path, by checking for the
// workspace's folder name. Best-effort: if the title couldn't be read, we
// can't disambiguate between multiple windows of the same IDE, so this fails
// open — an app-level focus match is treated as sufficient rather than
// silently going dark whenever the window-title API doesn't cooperate.
func windowTitleMatchesWorkspace(windowTitle, workspace string) bool {
	if windowTitle == "" {
		return true
	}
	base := strings.ToLower(filepath.Base(strings.TrimRight(workspace, "/\\")))
	if base == "" {
		return true
	}
	return strings.Contains(windowTitle, base)
}
