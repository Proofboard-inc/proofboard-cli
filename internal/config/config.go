package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultAPIBaseURL   = "https://api-dev.proofboard.io"
	DefaultAppBaseURL   = "https://proofboard-frontend.vercel.app"
	DefaultAgentAuthURL = DefaultAppBaseURL + "/cli-auth"
)

type Config struct {
	APIBaseURL                string
	AppBaseURL                string
	ReleaseBaseURL            string
	LinkPath                  string
	CheckPath                 string
	SyncPath                  string
	DeviceKeyRegistrationPath string
	RefreshPath               string
	AgentAuthURL              string
	DictionaryPath            string
	LatestVersionPath         string
	LogLevel                  string
	DefaultProductionBranches []string
}

func Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	v := viper.New()
	v.SetEnvPrefix("proofboard")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("api.base_url", DefaultAPIBaseURL)
	v.SetDefault("app.base_url", DefaultAppBaseURL)
	v.SetDefault("release.base_url", "https://proofboard.io")
	v.SetDefault("api.link_path", "/api/v1/cli/repos/link")
	v.SetDefault("api.check_path", "/api/v1/cli/repos/check")
	v.SetDefault("api.sync_path", "/api/v1/cli/sync")
	v.SetDefault("api.device_key_registration_path", "/api/v1/cli/auth/device-key")
	v.SetDefault("api.refresh_path", "/api/v1/cli/auth/refresh")
	v.SetDefault("agent.auth_url", DefaultAgentAuthURL)
	v.SetDefault("api.dictionary_path", "/api/v1/cli/dictionary")
	v.SetDefault("release.latest_version_path", "/latest.json")
	v.SetDefault("log.level", "info")
	v.SetDefault("git.production_branches", []string{"main", "master", "production"})
	apiBaseURL := strings.TrimRight(v.GetString("api.base_url"), "/")
	return Config{
		APIBaseURL:                apiBaseURL,
		AppBaseURL:                v.GetString("app.base_url"),
		ReleaseBaseURL:            v.GetString("release.base_url"),
		LinkPath:                  v.GetString("api.link_path"),
		CheckPath:                 v.GetString("api.check_path"),
		SyncPath:                  v.GetString("api.sync_path"),
		DeviceKeyRegistrationPath: v.GetString("api.device_key_registration_path"),
		RefreshPath:               v.GetString("api.refresh_path"),
		AgentAuthURL:              v.GetString("agent.auth_url"),
		DictionaryPath:            v.GetString("api.dictionary_path"),
		LatestVersionPath:         v.GetString("release.latest_version_path"),
		LogLevel:                  v.GetString("log.level"),
		DefaultProductionBranches: v.GetStringSlice("git.production_branches"),
	}, nil
}
