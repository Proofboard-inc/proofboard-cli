package commands

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestRootIncludesRequiredCommands(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := NewRootCommand(context.Background(), &out, &out)
	required := map[string]bool{
		"auth": false, "link": false, "unlink": false, "sync": false,
		"status": false, "logs": false, "update": false, "update-dictionary": false, "config": false, "agent": false,
		"completion": false, "detect": false, "install": false, "uninstall": false, "version": false,
	}
	for _, child := range cmd.Commands() {
		if _, ok := required[child.Name()]; ok {
			required[child.Name()] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("missing command %q", name)
		}
	}
}

func TestRelativeSyncTime(t *testing.T) {
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "recent", at: now.Add(-20 * time.Second), want: "just now"},
		{name: "minute", at: now.Add(-time.Minute), want: "1 minute ago"},
		{name: "minutes", at: now.Add(-3 * time.Minute), want: "3 minutes ago"},
		{name: "hour", at: now.Add(-time.Hour), want: "1 hour ago"},
		{name: "days", at: now.Add(-48 * time.Hour), want: "2 days ago"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := relativeSyncTime(now, test.at); got != test.want {
				t.Fatalf("relativeSyncTime() = %q, want %q", got, test.want)
			}
		})
	}
}
