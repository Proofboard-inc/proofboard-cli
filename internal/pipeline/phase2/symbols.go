package phase2

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Language-aware regex sets for extracting symbol-like identifiers
// (function/class/method names, import module names) from a commit's message
// body. This is a coarse, best-effort signal derived from prose the author
// wrote — not real diff/AST parsing (explicitly out of scope per the
// pure-Go, no-parser-dependency decision) — used only to add a small amount
// of extra classification signal beyond the subject line and file paths.
//
// Patterns are matched against an already-lowercased byte slice (see
// ExtractSymbols), so keywords in each pattern are lowercase.
// universalSymbolPatterns apply regardless of file extension: commit bodies
// are prose describing code ("adds chargeCard() and imports stripe"), not
// real source syntax, so a bare "identifier()" reference is a more reliable
// signal in body text than requiring a language keyword (func/def/function)
// to literally appear.
var universalSymbolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b([a-z_][a-z0-9_]*)\(\)`),
}

var jsLikeSymbolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bfunction\s+(\w+)\s*\(`),
	regexp.MustCompile(`\bconst\s+(\w+)\s*=`),
	regexp.MustCompile(`\bclass\s+(\w+)\b`),
	regexp.MustCompile(`\bimport\s+.*from\s+['"]([\w@./-]+)['"]`),
	regexp.MustCompile(`\brequire\(['"]([\w@./-]+)['"]\)`),
}

var symbolPatternsByExtension = map[string][]*regexp.Regexp{
	".go": {
		regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?(\w+)\s*\(`),
		regexp.MustCompile(`\btype\s+(\w+)\s+(?:struct|interface)\b`),
	},
	".ts":  jsLikeSymbolPatterns,
	".tsx": jsLikeSymbolPatterns,
	".js":  jsLikeSymbolPatterns,
	".jsx": jsLikeSymbolPatterns,
	".py": {
		regexp.MustCompile(`\bdef\s+(\w+)\s*\(`),
		regexp.MustCompile(`\bclass\s+(\w+)\b`),
		regexp.MustCompile(`\bimport\s+(\w+)\b`),
	},
	".rb": {
		regexp.MustCompile(`\bdef\s+(\w+)\b`),
		regexp.MustCompile(`\bclass\s+(\w+)\b`),
	},
	".java": {
		regexp.MustCompile(`\bclass\s+(\w+)\b`),
		regexp.MustCompile(`\bvoid\s+(\w+)\s*\(`),
	},
	".rs": {
		regexp.MustCompile(`\bfn\s+(\w+)\s*\(`),
		regexp.MustCompile(`\bstruct\s+(\w+)\b`),
	},
}

// ExtractSymbols scans an already-lowercased commit body for symbol-like
// identifiers, using regex sets selected by the file extensions present in
// filePaths. Only matched identifiers (short substrings) are copied into new
// strings — bodyLower itself is never copied wholesale, and the caller
// remains responsible for zeroing bodyLower after use (see intent.go), same
// as Subject.
func ExtractSymbols(bodyLower []byte, filePaths []string) []string {
	if len(bodyLower) == 0 || len(filePaths) == 0 {
		return nil
	}

	seenExt := make(map[string]bool)
	patterns := append([]*regexp.Regexp(nil), universalSymbolPatterns...)
	for _, path := range filePaths {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" || seenExt[ext] {
			continue
		}
		seenExt[ext] = true
		if p, ok := symbolPatternsByExtension[ext]; ok {
			patterns = append(patterns, p...)
		}
	}
	if len(patterns) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var symbols []string
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllSubmatch(bodyLower, -1) {
			if len(match) < 2 || len(match[1]) == 0 {
				continue
			}
			symbol := string(match[1])
			if seen[symbol] {
				continue
			}
			seen[symbol] = true
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}
