package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/spf13/cobra"
)

func newUpdateDictionaryCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "update-dictionary",
		Short: "Check for a newer category dictionary",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("update-dictionary: %w", err)
			}
			local, err := dictionary.LoadDefault(ctx)
			if err != nil {
				return err
			}
			releases := api.NewReleaseClient(runtime.config.ReleaseBaseURL)
			latest, err := releases.Latest(ctx, runtime.config.LatestDictionaryPath)
			if err != nil {
				return err
			}
			if latest.Version == "" || latest.Version == local.Version {
				_, err := fmt.Fprintf(out, "Dictionary is up to date (%s).\n", local.Version)
				return err
			}
			_, err = fmt.Fprintf(out, "A new dictionary is available: %s. Download: %s\n", latest.Version, latest.URL)
			return err
		},
	}
}
