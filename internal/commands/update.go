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
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

type updateCommandOptions struct {
	executablePath func() (string, error)
	install        func(io.Writer) error
}

func newUpdateCommand(ctx context.Context, out io.Writer) *cobra.Command {
	return newUpdateCommandWithOptions(ctx, out, updateCommandOptions{
		executablePath: os.Executable,
		install:        performInstall,
	})
}

func newUpdateCommandWithOptions(ctx context.Context, out io.Writer, options updateCommandOptions) *cobra.Command {
	if options.executablePath == nil {
		options.executablePath = os.Executable
	}
	if options.install == nil {
		options.install = performInstall
	}
	return &cobra.Command{
		Use:   "update",
		Short: "Update Proofboard Career Agent",
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
				_, err := fmt.Fprintf(out, "Proofboard Career Agent is up to date (%s).\n", version.Version)
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
			execPath, err := options.executablePath()
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

			// Download signature
			sigFile, err := os.CreateTemp(execDir, "proofboard-sig-*.tmp")
			if err != nil {
				_ = os.Remove(tempPath)
				return fmt.Errorf("create temp sig file: %w", err)
			}
			sigPath := sigFile.Name()
			err = releases.Download(ctx, downloadURL+".sig", sigFile)
			sigFile.Close()
			if err != nil {
				_ = os.Remove(tempPath)
				_ = os.Remove(sigPath)
				return fmt.Errorf("download binary signature: %w", err)
			}

			// Verify signature
			binData, err := os.ReadFile(tempPath)
			if err != nil {
				_ = os.Remove(tempPath)
				_ = os.Remove(sigPath)
				return fmt.Errorf("read downloaded binary: %w", err)
			}
			sigData, err := os.ReadFile(sigPath)
			if err != nil {
				_ = os.Remove(tempPath)
				_ = os.Remove(sigPath)
				return fmt.Errorf("read downloaded signature: %w", err)
			}
			if err := crypto.VerifyReleaseSignature(binData, sigData); err != nil {
				_ = os.Remove(tempPath)
				_ = os.Remove(sigPath)
				return fmt.Errorf("verify binary signature: %w", err)
			}
			_ = os.Remove(sigPath)

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

			// Ensure it's in PATH
			if err := options.install(out); err != nil {
				fmt.Fprintf(out, "Warning: Failed to perform system PATH installation: %v\n", err)
			}

			_, err = fmt.Fprintf(out, "Proofboard Career Agent updated successfully to version %s.\n", latest.Version)
			return err
		},
	}
}
