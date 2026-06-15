package commands

import (
	"bytes"
	"context"
	"testing"
)

func TestActivityCommandStructure(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	cmd := newActivityCommand(context.Background(), &out)
	if cmd.Use != "activity" {
		t.Errorf("expected use 'activity', got %s", cmd.Use)
	}

	// check if aliases exist
	foundAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "activity-log" {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Errorf("expected alias 'activity-log' not found")
	}
}
