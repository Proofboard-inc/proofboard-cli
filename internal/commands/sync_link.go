package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
	"github.com/proofboard/proofboard/internal/notifications"
	statestore "github.com/proofboard/proofboard/internal/state"
)

// errSkipSync signals that the caller should return nil immediately without
// running the rest of the sync, used when ensureRepoLinked determines there
// is nothing to do (a suppressed workspace, or an unlinked repo reached from
// a hook).
var errSkipSync = errors.New("skip sync")

// ensureRepoLinked auto-links a repo that isn't connected yet by running
// `proofboard link` non-interactively, and separately recovers a stale or
// missing EmailHashKey the same way. Returns the possibly-updated state and
// repo state, and errSkipSync when the caller should stop without erroring.
func ensureRepoLinked(ctx context.Context, out io.Writer, runtime runtimeContext, identity model.RemoteIdentity, triggerSource string, fromHook, fromAgent bool, repo pbgit.Repo, current model.State, repoState model.LinkedRepoState, linked bool) (model.State, model.LinkedRepoState, error) {
	if !linked {
		repoPath, err := filepath.Abs(repo.Path)
		if err != nil {
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "failure", err.Error())
			return current, repoState, err
		}
		if statestore.IsWorkspaceSuppressed(current, repoPath) {
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "workspace suppressed")
			return current, repoState, errSkipSync
		}
		if fromHook {
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "unlinked repo in hook")
			return current, repoState, errSkipSync
		}
		notifications.PrintEvent(out, notifications.NewProjectDetected(identity.Repo))
		fmt.Fprintln(out, "Preparing this project for automatic tracking...")
		linkCmd := newLinkCommand(ctx, out)
		if fromAgent {
			linkCmd.SetArgs([]string{"--non-interactive"})
		} else {
			linkCmd.SetArgs([]string{})
		}
		if err := linkCmd.ExecuteContext(ctx); err != nil {
			// Both of these are intentional non-links decided during the
			// ownership prompt, not failures: a public project is connected
			// through the dashboard instead of the CLI, and a declined
			// personal project simply isn't added to the career record.
			// Either way there's nothing to sync yet, so this reaches
			// errSkipSync exactly like a suppressed workspace does.
			if errors.Is(err, errPublicProjectNotLinked) {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "public project, connect via dashboard")
				return current, repoState, errSkipSync
			}
			if errors.Is(err, errPersonalProjectDeclined) {
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource, "link check", "skipped", "personal project declined")
				return current, repoState, errSkipSync
			}
			return current, repoState, err
		}
		var loadErr error
		current, loadErr = runtime.state.Load(ctx)
		if loadErr != nil {
			return current, repoState, loadErr
		}
		var ok bool
		repoState, ok = current.LinkedRepos[identity.RepoHash]
		if !ok {
			return current, repoState, fmt.Errorf("project connection did not complete")
		}
	}
	if repoState.EmailHashKey == "" {
		// Legacy state predates per-project email HMAC keys. Recover it
		// through the same documented flow as `proofboard link`; sending
		// the stale project ID as the initial request can return 404
		// after the backend's repository mapping has changed.
		linkCmd := newLinkCommand(ctx, out)
		linkCmd.SetArgs([]string{"--non-interactive"})
		if err := linkCmd.ExecuteContext(ctx); err != nil {
			return current, repoState, fmt.Errorf("refresh project security keys: %w", err)
		}
		var loadErr error
		current, loadErr = runtime.state.Load(ctx)
		if loadErr != nil {
			return current, repoState, fmt.Errorf("reload project security keys: %w", loadErr)
		}
		var refreshed bool
		repoState, refreshed = current.LinkedRepos[identity.RepoHash]
		if !refreshed || repoState.EmailHashKey == "" {
			return current, repoState, fmt.Errorf("refresh project security keys: link response did not include emailHashKey")
		}
	}
	return current, repoState, nil
}
