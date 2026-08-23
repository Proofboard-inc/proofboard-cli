package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/proofboard/proofboard/internal/api"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/logging"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func newUnlinkCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "unlink",
		Short: "Unlink the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("unlink: %w", err)
			}
			repo, err := pbgit.Discover(ctx, runtime.workingDir)
			if err != nil {
				return err
			}
			remoteURL, err := pbgit.OriginURL(ctx, repo)
			if err != nil {
				return err
			}
			identity, err := pbgit.ParseRemote(remoteURL)
			if err != nil {
				return err
			}
			unlinkErr := retryAfterAuth(ctx, out, "repository unlink", func() error {
				freshCredentials, credErr := runtime.credentials.Load(ctx)
				if credErr != nil {
					return fmt.Errorf("reload credentials: %w", credErr)
				}
				if freshCredentials.Token == "" {
					return fmt.Errorf("missing authentication token")
				}
				_, apiErr := runtime.api.UnlinkRepo(ctx, freshCredentials.Token, identity.RepoHash)
				return apiErr
			})
			// A 404 means the backend already has nothing to unlink (e.g. the
			// project was deleted some other way), which is a genuine success
			// case, not a failure. Anything else (network failure, an auth
			// failure that survived the full retry chain, a 5xx) means the
			// backend was never actually notified, so the outcome is tracked
			// rather than discarded, since telling the user it worked when the
			// dashboard project could still show as CLI-active would be wrong.
			backendConfirmed := unlinkErr == nil || api.IsStatus(unlinkErr, http.StatusNotFound)
			if !backendConfirmed {
				// Full error (URL, repo hash, underlying dial/DNS failure) goes to
				// the log, not the terminal: the person doesn't need "dial tcp:
				// lookup api-dev.proofboard.io: no such host" printed at them to
				// understand "couldn't reach the server".
				_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, "unlink", "confirm unlink with backend", "failure", unlinkErr.Error())
			}

			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			current = statestore.RemoveLinkedRepo(current, identity.RepoHash)
			// Disconnecting is a deliberate choice to stop tracking, so the
			// workspace becomes eligible for the connection prompt again.
			// Local state is cleared unconditionally either way: local-first,
			// there's nothing to gain by keeping local tracking alive just
			// because the backend couldn't be reached.
			if current, err = statestore.ClearWorkspacePrompt(current, repo.Path); err != nil {
				return err
			}
			if err := hooks.Uninstall(ctx, repo); err != nil {
				return err
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			if backendConfirmed {
				_, err = fmt.Fprintln(out, "Repository unlinked. Hooks removed.")
				return err
			}
			_, err = fmt.Fprintln(out, "Repository unlinked locally. Hooks removed.\n"+
				"Warning: Proofboard could not be reached to confirm the unlink — your dashboard may still show this\n"+
				"project as CLI-active until connectivity is restored. Run `proofboard unlink` again once you're back online.")
			return err
		},
	}
}
