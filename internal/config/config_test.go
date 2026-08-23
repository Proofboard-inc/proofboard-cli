package config

import (
	"context"
	"strings"
	"testing"
)

func TestLoadUsesShippedProductionDefaults(t *testing.T) {
	t.Setenv("PROOFBOARD_API_BASE_URL", "")
	t.Setenv("PROOFBOARD_APP_BASE_URL", "")
	t.Setenv("PROOFBOARD_AGENT_AUTH_URL", "")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// Asserted against the declared defaults rather than literal URLs: the
	// contract here is that an unconfigured machine gets the shipped
	// defaults, which is what breaks if the wiring regresses. Pinning the
	// literals instead only records which environment was current the day the
	// test was written, and has to be edited every time that changes.
	if cfg.APIBaseURL != DefaultAPIBaseURL {
		t.Fatalf("APIBaseURL = %q, want the default %q", cfg.APIBaseURL, DefaultAPIBaseURL)
	}
	if cfg.AppBaseURL != DefaultAppBaseURL {
		t.Fatalf("AppBaseURL = %q, want the default %q", cfg.AppBaseURL, DefaultAppBaseURL)
	}
	if cfg.AgentAuthURL != DefaultAgentAuthURL {
		t.Fatalf("AgentAuthURL = %q, want the default %q", cfg.AgentAuthURL, DefaultAgentAuthURL)
	}
	// The shipped defaults must be production, not a development host: a
	// release that quietly points at a preview environment sends real users'
	// data somewhere it does not belong.
	for name, got := range map[string]string{"APIBaseURL": cfg.APIBaseURL, "AppBaseURL": cfg.AppBaseURL} {
		if strings.Contains(got, "-dev.") || strings.Contains(got, "vercel.app") || strings.Contains(got, "onrender.com") {
			t.Errorf("%s = %q, which is a development host", name, got)
		}
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
