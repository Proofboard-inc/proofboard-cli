package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update local Proofboard CLI binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeContext, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			releases := api.NewReleaseClient(runtimeContext.config.ReleaseBaseURL)
			latest, err := releases.Latest(ctx, runtimeContext.config.LatestVersionPath)
			if err != nil {
				return err
			}
			if latest.Version == "" || latest.Version == version.Version {
				_, err := fmt.Fprintf(out, "Proofboard CLI is up to date (%s).\n", version.Version)
				return err
			}

			// Get binary name for current platform
			suffix := ""
			if runtime.GOOS == "windows" {
				suffix = ".exe"
			}
			binaryName := fmt.Sprintf("proofboard-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix)

			// Clean/build download URL
			downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(runtimeContext.config.ReleaseBaseURL, "/"), latest.Version, binaryName)

			// Get current running executable path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("retrieve executable path: %w", err)
			}
			execDir := filepath.Dir(execPath)

			// Create temp file in the same directory as the executable
			tempFile, err := os.CreateTemp(execDir, "proofboard-update-*.tmp")
			if err != nil {
				return fmt.Errorf("create temp file: %w", err)
			}
			tempPath := tempFile.Name()

			// Download new binary
			err = releases.Download(ctx, downloadURL, tempFile)
			tempFile.Close() // close file handle before renaming/chmod
			if err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("download new binary: %w", err)
			}

			// Set executable permissions
			if err := os.Chmod(tempPath, 0755); err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("set executable permissions: %w", err)
			}

			// Atomically rename
			if err := os.Rename(tempPath, execPath); err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("replace executable binary: %w", err)
			}

			_, err = fmt.Fprintf(out, "Proofboard CLI updated successfully to version %s.\n", latest.Version)
			return err
		},
	}
}
