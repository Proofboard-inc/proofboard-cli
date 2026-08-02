package commands

import (
	"io"

	"github.com/proofboard/proofboard/internal/spinner"
)

// withSpinner runs op with a labeled terminal spinner around it. The spinner
// itself already no-ops on non-TTY output; enabled additionally gates it on
// command-level context (e.g. --non-interactive, or a hook/agent-triggered
// sync) so agent/hook runs never get spinner output even if their stdout
// happens to be attached to a terminal.
func withSpinner(out io.Writer, label string, enabled bool, op func() error) error {
	if !enabled {
		return op()
	}
	sp := spinner.New(out, label)
	sp.Start()
	err := op()
	sp.Stop("")
	return err
}
