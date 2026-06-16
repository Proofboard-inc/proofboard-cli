package commands

import (
	"context"
	"fmt"
	"io"
	"sort"

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
			hashes := make([]string, 0, len(current.LinkedRepos))
			for repoHash := range current.LinkedRepos {
				hashes = append(hashes, repoHash)
			}
			sort.Strings(hashes)
			for _, repoHash := range hashes {
				repo := current.LinkedRepos[repoHash]
				fmt.Fprintf(out, "%s tier=%s lastSync=%s lastHead=%s\n", repoHash, repo.Tier, repo.LastSyncAt.Format("2006-01-02T15:04:05Z07:00"), repo.LastHeadSHA)
			}
			if err := triggerMonthlyCareerSummary(ctx, out, runtime); err != nil {
				return err
			}
			return nil
		},
	}
}
