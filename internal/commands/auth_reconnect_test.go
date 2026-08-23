package commands

import (
	"context"
	"strings"
	"testing"
)

// The automatic reconnect runs precisely because the server rejected the
// credentials on disk. If it invokes sign-in without forcing, the command
// sees a token present, reports "already authenticated", returns success
// without signing in, and the caller retries with the same rejected token —
// so an expired session can never recover on its own.
func TestReconnectForcesAFreshSignIn(t *testing.T) {
	cmd := newAuthCommand(context.Background(), nil)
	if cmd.Flags().Lookup("force") == nil {
		t.Fatal("auth has no --force flag, so the reconnect cannot bypass the already-authenticated short-circuit")
	}
	src := readSource(t, "auth_retry.go")
	if !strings.Contains(src, `args := []string{"--force"}`) {
		t.Error("runAuthFlow does not pass --force; a stale session will short-circuit instead of re-authenticating")
	}
}

// The short-circuit must still apply to a plain `proofboard auth`, which is
// what stops an already-connected developer being sent through a browser
// round trip for nothing.
func TestPlainAuthStillShortCircuits(t *testing.T) {
	src := readSource(t, "auth.go")
	if !strings.Contains(src, "if !switchAccount && !rotateKey && !forceReauth {") {
		t.Error("the already-authenticated short-circuit no longer guards plain `proofboard auth`")
	}
}
