package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/version"
	"github.com/spf13/cobra"
)

type updateCommandOptions struct {
	executablePath func() (string, error)
	install        func(io.Writer) error
}

const (
	proofboardReleaseBaseURL = "https://proofboard.io"
	proofboardGitHubRepo     = "Proofboard-inc/proofboard-cli"
)

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
		Short: "Update Proofboard Career Agent to the latest published release",
		Long: "Checks for a newer Proofboard Career Agent release and, if one exists, downloads it,\n" +
			"verifies its signature, and replaces the currently running binary in place.\n" +
			"Safe to run any time — a no-op if you're already on the latest version.",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeContext, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			releases := api.NewReleaseClient(runtimeContext.config.ReleaseBaseURL)
			latest, err := releases.Latest(ctx, runtimeContext.config.LatestVersionPath)
			var githubRelease api.GitHubRelease
			if err != nil && strings.TrimSuffix(runtimeContext.config.ReleaseBaseURL, "/") == proofboardReleaseBaseURL {
				// Keep proofboard.io as the canonical release origin, but allow
				// updates to continue from the directly published GitHub assets
				// when the root-domain manifest is temporarily unavailable.
				githubRelease, err = api.LatestGitHubRelease(ctx, proofboardGitHubRepo)
				if err == nil {
					manifestURL, found := githubRelease.AssetURL("latest.json")
					if !found {
						err = fmt.Errorf("GitHub latest release does not contain latest.json")
					} else {
						githubReleases := api.NewReleaseClient("https://api.github.com")
						latest, err = githubReleases.Latest(ctx, manifestURL)
						if err == nil {
							releases = githubReleases
						}
					}
				}
			}
			if errors.Is(err, api.ErrNoGitHubRelease) {
				_, printErr := fmt.Fprintf(out,
					"No published Proofboard Career Agent release was found yet — you're running the development build (%s).\n"+
						"Nothing to update to right now; check back once a release is published.\n",
					version.Version,
				)
				return printErr
			}
			if err != nil {
				return fmt.Errorf("check for a newer release: %w", err)
			}
			latestVersion := strings.TrimPrefix(latest.Version, "v")
			versionComparison, err := dictionary.CompareVersions(latestVersion, version.Version)
			if err != nil {
				return fmt.Errorf("compare Career Agent versions: %w", err)
			}
			if versionComparison <= 0 {
				_, err := fmt.Fprintf(out, "Proofboard Career Agent is up to date (%s).\n", version.Version)
				return err
			}

			// Get binary name for current platform
			suffix := ""
			if runtime.GOOS == "windows" {
				suffix = ".exe"
			}
			// Preferred name first, legacy name second. Release assets carry
			// both: the product-named file matches every installer on the
			// release page, and the lowercase one is what versions up to
			// 1.13.2 look for, so dropping it would strand them with no way
			// to update. Once those versions are gone the legacy name can go
			// with them.
			binaryName := releaseBinaryName(runtime.GOOS, runtime.GOARCH, suffix)
			legacyBinaryName := legacyReleaseBinaryName(runtime.GOOS, runtime.GOARCH, suffix)

			// Clean/build download URL
			downloadURL := strings.TrimSpace(latest.URL)
			signatureURL := ""
			// Set only on the plain download-host path, where there is no
			// asset list to consult: the preferred name is tried first and
			// this is what a 404 falls back to. A host that only carries the
			// old name must keep working.
			legacyDownloadURL := ""
			if len(githubRelease.Assets) > 0 {
				var found bool
				resolved := binaryName
				downloadURL, found = githubRelease.AssetURL(binaryName)
				if !found {
					resolved = legacyBinaryName
					downloadURL, found = githubRelease.AssetURL(legacyBinaryName)
				}
				if !found {
					return fmt.Errorf("GitHub latest release contains neither %s nor %s", binaryName, legacyBinaryName)
				}
				// The signature must belong to the same file that was
				// resolved, or verification compares a binary against another
				// build's signature and fails for a reason nobody can read.
				signatureURL, found = githubRelease.AssetURL(resolved + ".sig")
				if !found {
					return fmt.Errorf("GitHub latest release does not contain %s.sig", resolved)
				}
			} else if downloadURL == "" {
				base := strings.TrimSuffix(runtimeContext.config.ReleaseBaseURL, "/")
				downloadURL = fmt.Sprintf("%s/%s/%s", base, latest.Version, binaryName)
				legacyDownloadURL = fmt.Sprintf("%s/%s/%s", base, latest.Version, legacyBinaryName)
			} else if strings.HasSuffix(downloadURL, "/"+binaryName) ||
				strings.HasSuffix(downloadURL, "/"+legacyBinaryName) {
				// The manifest already names a specific file under either
				// naming, so it is used as given. Appending a name here would
				// build a path with the binary in it twice.
			} else {
				base := strings.TrimSuffix(downloadURL, "/")
				downloadURL = base + "/" + binaryName
				legacyDownloadURL = base + "/" + legacyBinaryName
			}
			if signatureURL == "" {
				signatureURL = downloadURL + ".sig"
			}

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
			if err != nil && legacyDownloadURL != "" {
				// Preferred name absent on this host; fall back to the name
				// releases used up to 1.13.2. The signature must then come
				// from the same name, or verification compares mismatched
				// files.
				tempFile.Close()
				tempFile, err = os.Create(tempPath)
				if err == nil {
					downloadURL = legacyDownloadURL
					signatureURL = legacyDownloadURL + ".sig"
					err = releases.Download(ctx, downloadURL, tempFile)
				}
			}
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
			err = releases.Download(ctx, signatureURL, sigFile)
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

			_, err = fmt.Fprintf(out, "Proofboard Career Agent updated successfully to version %s.\n", latestVersion)
			return err
		},
	}
}

// releaseBinaryName is the executable's name on a release. It matches the
// installer packages, so everything on the release page reads as one product
// rather than two.
func releaseBinaryName(goos, goarch, suffix string) string {
	return fmt.Sprintf("Proofboard-Career-Agent-%s-%s%s", goos, goarch, suffix)
}

// legacyReleaseBinaryName is the name used up to and including 1.13.2.
// Releases still carry it because an installed copy of those versions builds
// this exact string to find its own update and fails outright when it is
// absent.
func legacyReleaseBinaryName(goos, goarch, suffix string) string {
	return fmt.Sprintf("proofboard-%s-%s%s", goos, goarch, suffix)
}
