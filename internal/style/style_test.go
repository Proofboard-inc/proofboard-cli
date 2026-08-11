package style

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnabledFalseForNonTerminalWriter(t *testing.T) {
	var buf bytes.Buffer
	if Enabled(&buf) {
		t.Fatalf("Enabled() = true for a non-terminal writer, want false")
	}
	if got := Success(&buf, "ok"); got != "ok" {
		t.Fatalf("Success() = %q, want plain text for non-terminal writer", got)
	}
}

func TestEnabledFalseWhenNoColorSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if got := Heading(&buf, "Title"); got != "Title" {
		t.Fatalf("Heading() = %q, want plain text when NO_COLOR is set", got)
	}
}

func TestClusterLineIncludesCategoryAndCommitCount(t *testing.T) {
	var buf bytes.Buffer
	line := ClusterLine(&buf, "API & Backend Services", "feature", "large", 45)
	if !strings.Contains(line, "API & Backend Services") {
		t.Fatalf("ClusterLine() = %q, missing category", line)
	}
	if !strings.Contains(line, "45 commits") {
		t.Fatalf("ClusterLine() = %q, missing commit count", line)
	}
	if !strings.Contains(line, "large") {
		t.Fatalf("ClusterLine() = %q, missing impact scale", line)
	}
}

func TestClusterLineSingularCommit(t *testing.T) {
	var buf bytes.Buffer
	line := ClusterLine(&buf, "Bug Fixes & Maintenance", "bugfix", "small", 1)
	if !strings.Contains(line, "1 commit,") {
		t.Fatalf("ClusterLine() = %q, want singular \"commit\"", line)
	}
}
