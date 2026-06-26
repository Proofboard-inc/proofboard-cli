package commands

import (
	"context"
	"fmt"
	"io"
	"sort"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/spf13/cobra"
)

func newStatusCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Proofboard status",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			if len(current.LinkedRepos) == 0 {
				_, err := fmt.Fprintln(out, "No linked repositories.")
				return err
			}

			// Detect current directory repo information
			var currentRepoHash string
			var localHeadSHA string
			var checkErr error

			repo, err := pbgit.Discover(ctx, runtime.workingDir)
			if err == nil {
				remoteURL, err := pbgit.OriginURL(ctx, repo)
				if err == nil {
					identity, err := pbgit.ParseRemote(remoteURL)
					if err == nil {
						currentRepoHash = identity.RepoHash
						localHeadSHA, checkErr = pbgit.Head(ctx, repo)
					} else {
						checkErr = err
					}
				} else {
					checkErr = err
				}
			} else {
				checkErr = err
			}

			hashes := make([]string, 0, len(current.LinkedRepos))
			for repoHash := range current.LinkedRepos {
				hashes = append(hashes, repoHash)
			}
			sort.Strings(hashes)
			for _, repoHash := range hashes {
				repoState := current.LinkedRepos[repoHash]
				pending := "unknown"
				if checkErr == nil && currentRepoHash != "" && repoHash == currentRepoHash {
					if localHeadSHA != repoState.LastHeadSHA {
						pending = "yes"
					} else {
						pending = "no"
					}
				}
				fmt.Fprintf(out, "%s projectID=%s lastSync=%s lastHead=%s pending=%s\n",
					repoHash,
					repoState.ProjectID,
					repoState.LastSyncAt.Format("2006-01-02T15:04:05Z07:00"),
					repoState.LastHeadSHA,
					pending,
				)
			}
			if err := triggerMonthlyCareerSummary(ctx, out, runtime); err != nil {
				return err
			}
			return nil
		},
	}
}
