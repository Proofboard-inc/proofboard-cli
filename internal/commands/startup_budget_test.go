package commands

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The startup checks used to share one deadline, spent in order, so whatever
// the version check took was taken from the dictionary check after it. The
// dictionary reported "context deadline exceeded" on every single command
// while being reachable and 34 KB in size. Each network check must carry its
// own deadline derived from the caller's context, not from a budget another
// call has already spent.
func TestStartupChecksDoNotShareOneDeadline(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	body := string(src)

	for _, want := range []string{
		"context.WithTimeout(ctx, versionCheckBudget)",
		"context.WithTimeout(ctx, dictionaryBudget)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s: a network check is still sharing another call's deadline", want)
		}
	}

	// Derived from ctx, never from the short local budget, or the sharing is
	// reintroduced under a different name.
	if regexp.MustCompile(`context\.WithTimeout\(checkCtx,`).MatchString(body) {
		t.Error("a check derives its deadline from checkCtx, so it inherits time already spent")
	}
}

// Both network checks run on every command, including the sync a git hook
// fires on every commit, so both are throttled. An unthrottled check is a
// network round trip the developer pays for on each invocation.
func TestBothNetworkChecksAreThrottled(t *testing.T) {
	src, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	body := string(src)
	for _, field := range []string{"LastVersionCheck", "LastDictionaryUpdateCheck"} {
		if !strings.Contains(body, field) {
			t.Errorf("%s is not consulted, so that check runs on every command", field)
		}
	}
}
