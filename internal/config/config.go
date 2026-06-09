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
	SyncPath                  string
	LatestVersionPath         string
	LatestDictionaryPath      string
	DictionaryDownloadPath    string
	LogLevel                  string
	AuthCallbackPort          int
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
	v.SetDefault("api.base_url", "https://api.proofboard.io")
	v.SetDefault("app.base_url", "https://app.proofboard.io")
	v.SetDefault("release.base_url", "https://releases.proofboard.io")
	v.SetDefault("api.link_path", "/cli/link")
	v.SetDefault("api.sync_path", "/cli/sync")
	v.SetDefault("release.latest_version_path", "/latest.json")
	v.SetDefault("release.latest_dictionary_path", "/dictionary/latest.json")
	v.SetDefault("release.dictionary_download_path", "/dictionary/%s/dictionary.json")
	v.SetDefault("log.level", "info")
	v.SetDefault("auth.callback_port", 9876)
	v.SetDefault("git.production_branches", []string{"main", "master", "production"})
	return Config{
		APIBaseURL:                v.GetString("api.base_url"),
		AppBaseURL:                v.GetString("app.base_url"),
		ReleaseBaseURL:            v.GetString("release.base_url"),
		LinkPath:                  v.GetString("api.link_path"),
		SyncPath:                  v.GetString("api.sync_path"),
		LatestVersionPath:         v.GetString("release.latest_version_path"),
		LatestDictionaryPath:      v.GetString("release.latest_dictionary_path"),
		DictionaryDownloadPath:    v.GetString("release.dictionary_download_path"),
		LogLevel:                  v.GetString("log.level"),
		AuthCallbackPort:          v.GetInt("auth.callback_port"),
		DefaultProductionBranches: v.GetStringSlice("git.production_branches"),
	}, nil
}
