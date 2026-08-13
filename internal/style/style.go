// Package style adds minimal ANSI color/weight to CLI output. Deliberately
// zero-dependency (raw escape codes plus a manual TTY check), matching this
// CLI's existing preference for small, self-contained features over pulling
// in a terminal-styling library (see internal/spinner's isTerminal, which
// uses the same os.ModeCharDevice technique instead of golang.org/x/term).
package style

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

// Enabled reports whether styled output should be emitted for w: it must be
// a real terminal, and neither NO_COLOR nor PROOFBOARD_NO_COLOR may be set.
func Enabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("PROOFBOARD_NO_COLOR") != "" {
		return false
	}
	return isTerminal(w)
}

func wrap(w io.Writer, code, text string) string {
	if !Enabled(w) {
		return text
	}
	return code + text + reset
}

// Success, Warn, Muted, Heading, and Accent color a piece of text for
// terminal output, falling back to plain text for non-TTY/NO_COLOR writers.
func Success(w io.Writer, text string) string { return wrap(w, green, text) }
func Warn(w io.Writer, text string) string    { return wrap(w, yellow, text) }
func Muted(w io.Writer, text string) string   { return wrap(w, gray, text) }
func Heading(w io.Writer, text string) string { return wrap(w, bold, text) }
func Accent(w io.Writer, text string) string  { return wrap(w, cyan, text) }

// Brand renders the "Proofboard" wordmark bold + cyan — the one-off,
// slightly heavier treatment reserved for naming the product itself in a
// startup banner, distinct from Heading/Accent's single-attribute styling.
func Brand(w io.Writer, text string) string { return wrap(w, bold+cyan, text) }

type fileLike interface {
	Stat() (os.FileInfo, error)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(fileLike)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ClusterLine formats one detected milestone cluster as a single colored
// progress line, meant to be printed live as each cluster is found during a
// sync — every category detected shows up as it's found, not just the first.
func ClusterLine(w io.Writer, category, impactType, impactScale string, commitCount int) string {
	impactColor := green
	switch strings.ToLower(impactType) {
	case "bugfix":
		impactColor = yellow
	case "refactor":
		impactColor = gray
	}
	tag := impactType
	if tag == "" {
		tag = "change"
	}
	commits := "commit"
	if commitCount != 1 {
		commits = "commits"
	}
	check := wrap(w, green, "✓")
	tagText := wrap(w, impactColor, "["+strings.ToLower(tag)+"]")
	scale := strings.ToLower(strings.TrimSpace(impactScale))
	if scale != "" {
		scale = ", " + scale
	}
	return fmt.Sprintf("  %s %s %s (%d %s%s)", check, tagText, category, commitCount, commits, scale)
}
