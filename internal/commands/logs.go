package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newLogsCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show local Proofboard logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("logs: %w", err)
			}
			file, err := os.Open(logPath(runtime.homeDir))
			if os.IsNotExist(err) {
				_, err := fmt.Fprintln(out, "No Proofboard logs found.")
				return err
			}
			if err != nil {
				return fmt.Errorf("open logs: %w", err)
			}
			defer file.Close()
			buffer := make([]string, 0, lines)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				buffer = append(buffer, scanner.Text())
				if len(buffer) > lines {
					buffer = buffer[1:]
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read logs: %w", err)
			}
			for _, line := range buffer {
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 100, "number of log lines to print")
	return cmd
}
