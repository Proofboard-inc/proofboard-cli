//go:build linux

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
)

func showWorkspaceAction(ctx context.Context, action WorkspaceAction) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	defer conn.Close()

	title, body, primary, secondary := workspaceActionLabels(action.Kind)
	if action.RepoName != "" {
		body = fmt.Sprintf("%s\n%s", body, action.RepoName)
	}

	done := make(chan struct{}, 1)
	notifier, err := notify.New(
		conn,
		notify.WithOnAction(func(sig *notify.ActionInvokedSignal) {
			if sig == nil {
				return
			}
			if sig.ActionKey == "sync" {
				_ = ActivateWorkspaceAction(ctx, "sync", action.Workspace)
			}
			select {
			case done <- struct{}{}:
			default:
			}
		}),
		notify.WithOnClosed(func(_ *notify.NotificationClosedSignal) {
			select {
			case done <- struct{}{}:
			default:
			}
		}),
	)
	if err != nil {
		return nil
	}
	defer notifier.Close()

	_, err = notifier.SendNotification(notify.Notification{
		AppName:       "Proofboard",
		Summary:       title,
		Body:          body,
		Actions:       []notify.Action{{Key: "sync", Label: primary}, {Key: "dismiss", Label: secondary}},
		ExpireTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
	}
	return nil
}
