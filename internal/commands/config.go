package commands

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

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
		Use:   "set key true|false",
		Short: "Set a Proofboard configuration value (auto-update, auto-update-dictionary, keychain-disabled)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("parse %s: %w", args[0], err)
			}
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			switch args[0] {
			case "auto-update-dictionary":
				current.AutoUpdateDictionary = value
			case "keychain-disabled":
				// Default (false) keeps the OS keychain in use. Setting
				// true is an explicit opt-out for environments where OS
				// keychain access isn't reachable; falls back to a
				// 0600-permission ~/.proofboard/device.key file instead.
				current.KeychainDisabled = value
			case "auto-update":
				// Phrased positively for the user ("auto-update false")
				// while the stored field is negative, so an older
				// state.json without the field still defaults to enabled.
				current.AutoUpdateCLIDisabled = !value
			default:
				return fmt.Errorf("unsupported config key %q", args[0])
			}
			if err := runtime.state.Save(ctx, current); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "%s=%t\n", args[0], value)
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
	cmd.AddCommand(&cobra.Command{
		Use:   "add-ide process-name",
		Short: "Add an IDE process for Career Agent workspace detection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("IDE process name cannot be empty")
			}
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			for _, existing := range current.IDEProcesses {
				if strings.EqualFold(existing, name) {
					return nil
				}
			}
			current.IDEProcesses = append(current.IDEProcesses, name)
			return runtime.state.Save(ctx, current)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove-ide process-name",
		Short: "Remove an IDE process from Career Agent workspace detection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			current, err := runtime.state.Load(ctx)
			if err != nil {
				return err
			}
			updated := current.IDEProcesses[:0]
			for _, existing := range current.IDEProcesses {
				if !strings.EqualFold(existing, args[0]) {
					updated = append(updated, existing)
				}
			}
			current.IDEProcesses = updated
			return runtime.state.Save(ctx, current)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "ides",
		Short: "Print IDE processes watched by the Career Agent",
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
			for _, name := range current.IDEProcesses {
				if _, err := fmt.Fprintln(out, name); err != nil {
					return err
				}
			}
			return nil
		},
	})
	return cmd
}
