package commands

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/style"
)

// reportSyncOutcome prints the final user-facing line(s) for a completed
// transmit, branching on the backend's receipt status. Shared by the normal
// sync flow and `sync --resync` so both surface identical wording for
// "deduped"/"regenerating" responses.
func reportSyncOutcome(out io.Writer, receipt model.SyncReceipt, payload model.SyncPayload, metadataOnly bool) error {
	switch receipt.Status {
	case "deduped", "duplicate":
		if _, err := fmt.Fprintln(out, "No new commits since your last sync (nothing changed on the server)."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "To regenerate your milestone summaries without new commits, run: proofboard sync --resync"); err != nil {
			return err
		}
	case "regenerating":
		if _, err := fmt.Fprintf(out, "%s %s %s\n",
			style.Success(out, "✓"),
			style.Brand(out, "Proofboard"),
			style.Heading(out, "— Regenerate requested for your milestone summaries. Refresh your dashboard shortly to see updated text.")); err != nil {
			return err
		}
	default:
		// Print one live line per detected cluster as they're found, so
		// every category synced this run is visible in real time. This is
		// informational only: the backend's clustering/AI-summary pass is
		// still async at this point, so there's nothing to review yet.
		// The actionable "ready to review" prompt surfaces later, once
		// that finishes, via the sync-complete notification (see
		// `proofboard notices`, wired into shell startup).
		for _, cluster := range payload.MilestoneClusters {
			fmt.Fprintln(out, style.ClusterLine(out, cluster.Category, cluster.ImpactType, cluster.ImpactScale, cluster.CommitCount))
		}
		var err error
		if len(payload.SHAs) == 0 && metadataOnly {
			_, err = fmt.Fprintf(out, "%s %s %s\n",
				style.Success(out, "✓"), style.Brand(out, "Proofboard"), style.Heading(out, "— Repository metadata synchronized."))
		} else {
			_, err = fmt.Fprintf(out, "%s %s %s\n",
				style.Success(out, "✓"), style.Brand(out, "Proofboard"),
				style.Heading(out, fmt.Sprintf("— Synced %d commits. Clusters detected: %d.", len(payload.SHAs), len(payload.MilestoneClusters))))
		}
		if err != nil {
			return err
		}
		if len(payload.MilestoneClusters) > 0 {
			if _, err := fmt.Fprintln(out, style.Muted(out, "Finishing analysis — check your dashboard shortly to review and publish.")); err != nil {
				return err
			}
		}
	}
	return nil
}

func isDocFile(filePath string) bool {
	lower := strings.ToLower(filepath.Base(filePath))
	if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".rst") {
		return true
	}
	if strings.HasPrefix(lower, "readme") || strings.HasPrefix(lower, "changelog") || strings.HasPrefix(lower, "license") {
		return true
	}
	return false
}

func isRevertSubject(subject []byte) bool {
	trimmed := bytes.TrimSpace(subject)
	prefix := [...]byte{'r', 'e', 'v', 'e', 'r', 't', ':'}
	return len(trimmed) >= len(prefix) && bytes.EqualFold(trimmed[:len(prefix)], prefix[:])
}

func abortSyncWithTrigger(homeDir, repoHash, triggerSource string) error {
	return logging.WriteSyncLog(homeDir, repoHash, triggerSource, "pre-classification filter", "aborted", "trivial merge skipped")
}
