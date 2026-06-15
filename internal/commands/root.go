package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func Execute(ctx context.Context, args []string) error {
	root := NewRootCommand(ctx, os.Stdout, os.Stderr)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("execute command: %w", err)
	}
	return nil
}

func NewRootCommand(ctx context.Context, out io.Writer, errOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "proofboard",
		Short:         "Local-first developer verification",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.AddCommand(
		newAuthCommand(ctx, out),
		newLinkCommand(ctx, out),
		newUnlinkCommand(ctx, out),
		newSyncCommand(ctx, out),
		newStatusCommand(ctx, out),
		newLogsCommand(ctx, out),
		newUpdateCommand(ctx, out),
		newUpdateDictionaryCommand(ctx, out),
		newConfigCommand(ctx, out),
		newNotificationsCommand(ctx, out),
		newActivityCommand(ctx, out),
	)
	return cmd
}
