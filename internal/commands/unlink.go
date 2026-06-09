package commands

import (
	"context"
	"fmt"
	"io"

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
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			current = statestore.RemoveLinkedRepo(current, identity.RepoHash)
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
