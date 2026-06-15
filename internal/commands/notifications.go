package commands

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newNotificationsCommand(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "Manage Proofboard notifications",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	var page int
	var limit int
	var unread bool
	var notifType string

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List notifications",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("notifications: %w", err)
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
			if unread {
				query.Set("isRead", "false")
			}
			if notifType != "" {
				query.Set("type", notifType)
			}
			res, err := runtime.api.GetNotifications(ctx, credentials.Token, query)
			if err != nil {
				return fmt.Errorf("get notifications: %w", err)
			}
			if len(res.Data) == 0 {
				fmt.Fprintln(out, "No notifications found.")
				return nil
			}
			for _, n := range res.Data {
				readStatus := "Read"
				if !n.IsRead {
					readStatus = "Unread"
				}
				fmt.Fprintf(out, "[%s] ID=%s Type=%s Date=%s Url=%s\n", readStatus, n.ID, n.Type, n.CreatedAt.Format("2006-01-02 15:04:05"), n.ActionURL)
			}
			return nil
		},
	}
	listCmd.Flags().IntVar(&page, "page", 1, "page number")
	listCmd.Flags().IntVar(&limit, "limit", 10, "limit per page")
	listCmd.Flags().BoolVar(&unread, "unread", false, "filter unread notifications only")
	listCmd.Flags().StringVar(&notifType, "type", "", "filter notifications by type")

	countCmd := &cobra.Command{
		Use:   "unread-count",
		Short: "Get unread notifications count",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("notifications: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			res, err := runtime.api.GetUnreadNotificationCount(ctx, credentials.Token)
			if err != nil {
				return fmt.Errorf("get unread count: %w", err)
			}
			fmt.Fprintf(out, "Unread count: %d\n", res.Count)
			return nil
		},
	}

	readCmd := &cobra.Command{
		Use:   "read <id>",
		Short: "Mark a notification as read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("notifications: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			err = runtime.api.MarkNotificationRead(ctx, credentials.Token, args[0])
			if err != nil {
				return fmt.Errorf("mark read: %w", err)
			}
			fmt.Fprintf(out, "Notification %s marked as read.\n", args[0])
			return nil
		},
	}

	markAllReadCmd := &cobra.Command{
		Use:   "mark-all-read",
		Short: "Mark all notifications as read",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("notifications: %w", err)
			}
			credentials, err := runtime.credentials.Load(ctx)
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			err = runtime.api.MarkAllNotificationsRead(ctx, credentials.Token)
			if err != nil {
				return fmt.Errorf("mark all read: %w", err)
			}
			fmt.Fprintln(out, "All notifications marked as read.")
			return nil
		},
	}

	cmd.AddCommand(listCmd, countCmd, readCmd, markAllReadCmd)
	return cmd
}
