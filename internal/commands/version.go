package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current version of the proofboard CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "proofboard version %s\n", version.Version)
		},
	}
}
