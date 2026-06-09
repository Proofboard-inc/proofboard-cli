package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/proofboard/proofboard/internal/crypto"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/hooks"
	"github.com/proofboard/proofboard/internal/model"
	statestore "github.com/proofboard/proofboard/internal/state"
	"github.com/spf13/cobra"
)

func newLinkCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "Link the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("link: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
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
			response, err := runtime.api.Link(ctx, credentials.Token, identity, credentials.EmailHash)
			if err != nil {
				return fmt.Errorf("register linked repository: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			head, _ := pbgit.Head(ctx, repo)
			current = statestore.AddLinkedRepo(current, model.LinkedRepoState{
				RepoHash:           identity.RepoHash,
				OrgHash:            identity.OrgHash,
				PathHash:           crypto.SHA256(repo.Path),
				Provider:           identity.Provider,
				LastHeadSHA:        head,
				LastSyncAt:         time.Time{},
				Tier:               response.Tier,
				DictionaryVersion:  "1.2.0",
				ProductionBranches: runtime.config.DefaultProductionBranches,
			})
			if err := hooks.Install(ctx, repo); err != nil {
				return err
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			display := response.DisplayOrg
			if display == "" {
				display = identity.Org
			}
			_, err = fmt.Fprintf(out, "Linked repository for %s. Hooks installed.\n", display)
			return err
		},
	}
}
