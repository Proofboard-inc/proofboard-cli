package commands

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
)

func newConfigCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Proofboard configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "set auto-update-dictionary true|false",
		Short: "Set dictionary auto-update behaviour",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "auto-update-dictionary" {
				return fmt.Errorf("unsupported config key %q", args[0])
			}
			value, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("parse auto-update-dictionary: %w", err)
			}
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			current.AutoUpdateDictionary = value
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "auto-update-dictionary=%t\n", value)
			return err
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add-branch name",
		Short: "Add a branch to watched branches",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			exists := false
			for _, b := range current.WatchedBranches {
				if b == branch {
					exists = true
					break
				}
			}
			if !exists {
				current.WatchedBranches = append(current.WatchedBranches, branch)
				if err := runtime.state.Save(ctx, current); err != nil {
					return err
				}
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove-branch name",
		Short: "Remove a branch from watched branches",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			var updated []string
			for _, b := range current.WatchedBranches {
				if b != branch {
					updated = append(updated, b)
				}
			}
			current.WatchedBranches = updated
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "branches",
		Short: "Print currently watched branches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			for _, b := range current.WatchedBranches {
				if _, err := fmt.Fprintln(out, b); err != nil {
					return err
				}
			}
			return nil
		},
	})
	return cmd
}
