package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestStatusJSONSupportsCareerAgentDashboard(t *testing.T) {
	setTestHome(t, t.TempDir())
	var out bytes.Buffer
	cmd := newStatusCommand(context.Background(), &out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var status struct {
		Product              string `json:"product"`
		Active               bool   `json:"active"`
		RepositoriesTracked  int    `json:"repositoriesTracked"`
		AuthenticationStatus string `json:"authenticationStatus"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v; output=%q", err, out.String())
	}
	if status.Product != "Proofboard Career Agent" || status.Active || status.RepositoriesTracked != 0 || status.AuthenticationStatus != "Not connected" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
