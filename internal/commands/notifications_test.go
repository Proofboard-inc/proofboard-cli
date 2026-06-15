package commands

import (
	"bytes"
	"context"
	"testing"
)

func TestNotificationsCommandStructure(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := newNotificationsCommand(context.Background(), &out)
	if cmd.Use != "notifications" {
		t.Errorf("expected use 'notifications', got %s", cmd.Use)
	}

	subCommands := map[string]bool{
		"list":          false,
		"unread-count":  false,
		"read":          false,
		"mark-all-read": false,
	}

	for _, sub := range cmd.Commands() {
		if _, ok := subCommands[sub.Name()]; ok {
			subCommands[sub.Name()] = true
		}
	}

	for name, found := range subCommands {
		if !found {
			t.Errorf("missing subcommand %s", name)
		}
	}
}
