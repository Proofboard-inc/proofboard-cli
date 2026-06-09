package commands

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update local Proofboard metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeContext, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			releases := api.NewReleaseClient(runtimeContext.config.ReleaseBaseURL)
			latest, err := releases.Latest(ctx, runtimeContext.config.LatestVersionPath)
			if err != nil {
				return err
			}
			if latest.Version == "" || latest.Version == version.Version {
				_, err := fmt.Fprintf(out, "Proofboard CLI is up to date (%s).\n", version.Version)
				return err
			}
			_, err = fmt.Fprintf(out, "A new version of Proofboard CLI is available: %s for %s/%s. Download: %s\n", latest.Version, runtime.GOOS, runtime.GOARCH, latest.URL)
			return err
		},
	}
}
