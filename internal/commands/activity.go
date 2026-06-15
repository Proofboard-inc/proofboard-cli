package commands

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newActivityCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var page int
	var limit int
	var actType string
	var dateFrom string
	var dateTo string

	cmd := &cobra.Command{
		Use:     "activity",
		Aliases: []string{"activity-log"},
		Short:   "View Proofboard activity log",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("activity: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			query := url.Values{}
			if page > 0 {
				query.Set("page", strconv.Itoa(page))
			}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(limit))
			}
			if actType != "" {
				query.Set("type", actType)
			}
			if dateFrom != "" {
				query.Set("dateFrom", dateFrom)
			}
			if dateTo != "" {
				query.Set("dateTo", dateTo)
			}
			res, err := runtime.api.GetActivityLog(ctx, credentials.Token, query)
			if err != nil {
				return fmt.Errorf("get activity log: %w", err)
			}
			if len(res.Data) == 0 {
				fmt.Fprintln(out, "No activity logs found.")
				return nil
			}
			for _, a := range res.Data {
				fmt.Fprintf(out, "Date=%s Type=%s ID=%s\n", a.CreatedAt.Format("2006-01-02 15:04:05"), a.Type, a.ID)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "page number")
	cmd.Flags().IntVar(&limit, "limit", 10, "limit per page")
	cmd.Flags().StringVar(&actType, "type", "", "filter activity by type")
	cmd.Flags().StringVar(&dateFrom, "from", "", "filter activity from date (ISO 8601)")
	cmd.Flags().StringVar(&dateTo, "to", "", "filter activity to date (ISO 8601)")
	return cmd
}
