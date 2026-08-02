package commands

import (
	"context"
	"fmt"
	"io"

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

			dictionaryURL := fmt.Sprintf("%s%s", runtime.config.APIBaseURL, runtime.config.DictionaryPath)
			result, err := dictionary.Update(ctx, runtime.homeDir, dictionaryURL, local)
			if err != nil {
				return err
			}
			if !result.Updated {
				_, err := fmt.Fprintf(out, "Dictionary is up to date (%s).\n", result.Version)
				return err
			}

			current, err := runtime.state.Load(ctx)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			current.DictionaryVersion = result.Version
			if err := runtime.state.Save(ctx, current); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			_, err = fmt.Fprintf(out, "Dictionary updated successfully to version %s.\n", result.Version)
			return err
		},
	}
}
