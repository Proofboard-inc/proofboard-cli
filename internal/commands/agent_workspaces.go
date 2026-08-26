package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	pbgit "github.com/proofboard/proofboard/internal/git"
)

func addWorkspaceCandidate(ctx context.Context, candidate string, seen map[string]bool, workspaces *[]string) {
	path := normalizeWorkspaceCandidate(candidate)
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		path = filepath.Dir(path)
	}
	repo, err := pbgit.Discover(ctx, path)
	if err != nil {
		return
	}
	repoPath, err := filepath.Abs(repo.Path)
	if err != nil || seen[repoPath] {
		return
	}
	seen[repoPath] = true
	*workspaces = append(*workspaces, repoPath)
}

// workspacePathFromFileURI converts an editor's "file://" folder URI into a
// filesystem path for the given operating system.
//
// The Windows case is the reason this is not a one-liner. VS Code writes the
// open folder as file:///c%3A/Users/ada/project, so url.Parse hands back the
// path /c:/Users/ada/project — with a leading slash that belongs to the URI
// rather than to the path, and forward slashes throughout. Passing that to
// filepath.Abs yields something like C:\c:\Users\ada\project, which matches
// no repository on the machine, and discovery then returns nothing at all —
// indistinguishable from having no editor open.
//
// goos is a parameter rather than read from runtime so both conventions can be
// exercised from any host.
func workspacePathFromFileURI(candidate, goos string) string {
	parsed, err := url.Parse(candidate)
	if err != nil || (parsed.Host != "" && parsed.Host != "localhost") {
		return ""
	}
	path := parsed.Path
	if goos != "windows" {
		return path
	}
	// "/C:/Users/..." -> "C:/Users/...". Checked rather than assumed: a URI
	// without a drive letter (a UNC share, say) must keep its leading slash.
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	// Converted explicitly rather than with filepath.FromSlash, which switches
	// on the HOST separator and is a no-op on Unix. This function takes goos as
	// a parameter precisely so the Windows conversion can be exercised from any
	// machine; using a host-dependent call here would have made that impossible
	// and left the bug undiscoverable anywhere but on Windows.
	return strings.ReplaceAll(path, "/", `\`)
}

func normalizeWorkspaceCandidate(candidate string) string {
	candidate = strings.Trim(strings.TrimSpace(candidate), "\"'")
	if candidate == "" || strings.HasPrefix(candidate, "-") {
		return ""
	}
	if strings.HasPrefix(candidate, "file://") {
		candidate = workspacePathFromFileURI(candidate, runtime.GOOS)
		if candidate == "" {
			return ""
		}
	}
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return ""
	}
	return absPath
}

func splitCommandLine(line string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	characters := []rune(line)
	for index, char := range characters {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			if index+1 < len(characters) {
				next := characters[index+1]
				if next == '\\' || next == '"' || next == '\'' || next == ' ' || next == '\t' {
					escaped = true
					continue
				}
			}
			current.WriteRune(char)
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' || char == '\n' {
			flush()
			continue
		}
		current.WriteRune(char)
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return fields
}

func discoverEditorStateWorkspaces(ctx context.Context, seen map[string]bool, workspaces *[]string) {
	for _, path := range editorStateFiles() {
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 8*1024*1024 {
			continue
		}
		var value any
		if json.Unmarshal(data, &value) != nil {
			continue
		}
		visitWorkspaceStrings(value, func(candidate string) {
			addWorkspaceCandidate(ctx, candidate, seen, workspaces)
		})
	}
}

func visitWorkspaceStrings(value any, visit func(string)) {
	switch typed := value.(type) {
	case string:
		visit(typed)
	case []any:
		for _, child := range typed {
			visitWorkspaceStrings(child, visit)
		}
	case map[string]any:
		for _, child := range typed {
			visitWorkspaceStrings(child, visit)
		}
	}
}

func editorStateFiles() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var roots []string
	switch runtime.GOOS {
	case "darwin":
		base := filepath.Join(homeDir, "Library", "Application Support")
		roots = []string{filepath.Join(base, "Code"), filepath.Join(base, "Code - Insiders"), filepath.Join(base, "Cursor")}
	case "windows":
		base := os.Getenv("APPDATA")
		roots = []string{filepath.Join(base, "Code"), filepath.Join(base, "Code - Insiders"), filepath.Join(base, "Cursor")}
	default:
		base := filepath.Join(homeDir, ".config")
		roots = []string{filepath.Join(base, "Code"), filepath.Join(base, "Code - Insiders"), filepath.Join(base, "Cursor")}
	}
	paths := make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, filepath.Join(root, "User", "globalStorage", "storage.json"))
	}
	return paths
}
