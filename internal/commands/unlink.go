package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/proofboard/proofboard/internal/api"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
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
			if unlinkErr != nil && !api.IsStatus(unlinkErr, http.StatusNotFound) {
				fmt.Fprintf(out, "Warning: could not notify Proofboard that this repo was unlinked (%v).\nHooks and local tracking have been removed regardless. Run `proofboard status`\nor check the dashboard if the project still shows as CLI-active.\n", unlinkErr)
			}

			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			current = statestore.RemoveLinkedRepo(current, identity.RepoHash)
			// Disconnecting is a deliberate choice to stop tracking, so the
			// workspace becomes eligible for the connection prompt again.
			if current, err = statestore.ClearWorkspacePrompt(current, repo.Path); err != nil {
				return err
			}
			if err := hooks.Uninstall(ctx, repo); err != nil {
				return err
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, "Repository unlinked. Hooks removed.")
			return err
		},
	}
}
