// Package spinner draws a terminal progress indicator for long-running
// network phases (link/sync). It is dependency-free, consistent with the
// CLI's no-third-party-dependency stance for small, self-contained features,
// and it never writes a partial frame when the destination is not a TTY —
// required so --json output, piped output, hook-triggered background syncs,
// and log files stay clean.
package spinner

import (
	"fmt"
	"io"
	"os"
	"time"
)

var frames = []rune{'|', '/', '-', '\\'}

const frameInterval = 100 * time.Millisecond

type Spinner struct {
	label   string
	out     io.Writer
	active  bool
	started bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// New creates a Spinner. It is inert (Start/Stop are no-ops) whenever out is
// not a TTY, so callers can construct and use one unconditionally.
func New(out io.Writer, label string) *Spinner {
	return &Spinner{
		label:  label,
		out:    out,
		active: isTerminal(out),
	}
}

// Start begins animating the spinner. No-op if inactive or already started.
func (s *Spinner) Start() {
	if s == nil || !s.active || s.started {
		return
	}
	s.started = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	go s.run()
}

func (s *Spinner) run() {
	defer close(s.doneCh)
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			fmt.Fprintf(s.out, "\r%c %s", frames[i%len(frames)], s.label)
			i++
		}
	}
}

// Stop halts the animation, clears the line, and — if finalMessage is
// non-empty — prints it on its own line. Safe to call even if Start was
// never called (e.g. non-interactive runs that skip Start entirely).
func (s *Spinner) Stop(finalMessage string) {
	if s == nil || !s.started {
		return
	}
	close(s.stopCh)
	<-s.doneCh
	s.started = false
	fmt.Fprint(s.out, "\r\033[K")
	if finalMessage != "" {
		fmt.Fprintln(s.out, finalMessage)
	}
}

type fileLike interface {
	Stat() (os.FileInfo, error)
}

// isTerminal reports whether w is a character device (a real terminal), not
// a pipe, file, or in-memory buffer. Manual os.ModeCharDevice check rather
// than a golang.org/x/term dependency, matching this CLI's preference for
// zero-dependency, self-contained small features.
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
