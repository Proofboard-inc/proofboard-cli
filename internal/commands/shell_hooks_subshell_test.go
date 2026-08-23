package commands

import (
	"strings"
	"testing"
)

// The chpwd install guard must NOT be exported. The hook function and the
// prompt wiring it installs are per-shell and are never inherited, so an
// exported guard makes every child shell believe the hook is already present
// and skip installing it — leaving directory detection dead in tmux panes,
// editor terminals and any nested shell. The startup guard
// (PROOFBOARD_DETECTED) is deliberately exported; this one must not be.
func TestChpwdInstallGuardIsNotExported(t *testing.T) {
	for name, hook := range map[string]string{
		"zsh":  zshChpwdHook,
		"bash": bashChpwdHook,
		"fish": fishChpwdHook,
	} {
		if strings.Contains(hook, "export PROOFBOARD_CHPWD_INSTALLED") ||
			strings.Contains(hook, "set -gx PROOFBOARD_CHPWD_INSTALLED") {
			t.Errorf("%s: chpwd guard is exported, so subshells skip installing the hook", name)
		}
		if !strings.Contains(hook, "PROOFBOARD_CHPWD_INSTALLED") {
			t.Errorf("%s: chpwd hook lost its install guard entirely", name)
		}
	}
}
