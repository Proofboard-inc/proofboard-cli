package config

import (
	"context"
	"testing"
)

func TestLoadUsesDevelopmentServiceDefaults(t *testing.T) {
	t.Setenv("PROOFBOARD_API_BASE_URL", "")
	t.Setenv("PROOFBOARD_APP_BASE_URL", "")
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", "")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIBaseURL != "https://api-dev.proofboard.io" {
		t.Fatalf("APIBaseURL = %q, want development backend", cfg.APIBaseURL)
	}
	if cfg.AppBaseURL != "https://proofboard-frontend.vercel.app" {
		t.Fatalf("AppBaseURL = %q, want development frontend", cfg.AppBaseURL)
	}
	if cfg.AgentAuthURL != "https://proofboard-frontend.vercel.app/cli-auth" {
		t.Fatalf("AgentAuthURL = %q, want development frontend auth route", cfg.AgentAuthURL)
	}
}

func TestLoadAllowsIndependentServiceOverrides(t *testing.T) {
	t.Setenv("PROOFBOARD_API_BASE_URL", "https://api.proofboard.io")
	t.Setenv("PROOFBOARD_APP_BASE_URL", "https://proofboard.io")
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", "https://proofboard.io/agent/cli-auth")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIBaseURL != "https://api.proofboard.io" {
		t.Fatalf("APIBaseURL = %q, want production backend override", cfg.APIBaseURL)
	}
	if cfg.AppBaseURL != "https://proofboard.io" {
		t.Fatalf("AppBaseURL = %q, want production frontend override", cfg.AppBaseURL)
	}
	if cfg.AgentAuthURL != "https://proofboard.io/agent/cli-auth" {
		t.Fatalf("AgentAuthURL = %q, want independent auth URL override", cfg.AgentAuthURL)
	}
}
