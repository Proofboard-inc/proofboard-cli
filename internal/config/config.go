package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	APIBaseURL                string
	AppBaseURL                string
	ReleaseBaseURL            string
	LinkPath                  string
	CheckPath                 string
	SyncPath                  string
	DeviceKeyRegistrationPath string
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
	v.SetDefault("api.base_url", "https://api-dev.proofboard.io")
	v.SetDefault("app.base_url", "https://app.proofboard.io")
	v.SetDefault("release.base_url", "https://releases.proofboard.io")
	v.SetDefault("api.link_path", "/api/v1/cli/repos/link")
	v.SetDefault("api.check_path", "/api/v1/cli/repos/check")
	v.SetDefault("api.sync_path", "/api/v1/cli/sync")
	v.SetDefault("api.device_key_registration_path", "/api/v1/cli/auth/device-key")
	v.SetDefault("api.dictionary_path", "/api/v1/cli/dictionary")
	v.SetDefault("release.latest_version_path", "/latest.json")
	v.SetDefault("log.level", "info")
	v.SetDefault("git.production_branches", []string{"main", "master", "production"})
	return Config{
		APIBaseURL:                v.GetString("api.base_url"),
		AppBaseURL:                v.GetString("app.base_url"),
		ReleaseBaseURL:            v.GetString("release.base_url"),
		LinkPath:                  v.GetString("api.link_path"),
		CheckPath:                 v.GetString("api.check_path"),
		SyncPath:                  v.GetString("api.sync_path"),
		DeviceKeyRegistrationPath: v.GetString("api.device_key_registration_path"),
		DictionaryPath:            v.GetString("api.dictionary_path"),
		LatestVersionPath:         v.GetString("release.latest_version_path"),
		LogLevel:                  v.GetString("log.level"),
		DefaultProductionBranches: v.GetStringSlice("git.production_branches"),
	}, nil
}
