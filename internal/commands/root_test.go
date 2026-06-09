package commands

import (
	"bytes"
	"context"
	"testing"
)

func TestRootIncludesRequiredCommands(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := NewRootCommand(context.Background(), &out, &out)
	required := map[string]bool{
		"auth": false, "link": false, "unlink": false, "sync": false,
		"status": false, "logs": false, "update": false, "update-dictionary": false, "config": false,
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
