package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/dictionary"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/version"
)

// autoUpdateInterval is how often the background agent looks for a newer
// executable. Daily: releases land far less often than that, and the check
// costs a single unauthenticated GitHub API call.
const autoUpdateInterval = 24 * time.Hour

// installScriptTimeout bounds the detached installer. The script downloads a
// ~10 MB binary and verifies its signature, so it is generous, but it must be
// finite: an installer wedged on a stalled download would otherwise sit there
// until the machine reboots.
const installScriptTimeout = 15 * time.Minute

// autoUpdateDisabled reports whether this machine has opted out. The env var
// is the escape hatch for CI images and locked-down fleets that must never
// have a binary replaced underneath them; the persisted flag is the
// documented per-machine setting.
func autoUpdateDisabled(state struct {
	Disabled bool
}) bool {
	return os.Getenv("PROOFBOARD_DISABLE_AUTO_UPDATE") == "1" || state.Disabled
}

// installScriptAsset names the release asset that installs or updates on this
// platform. The scripts are the same ones a first-time user runs, so there is
// exactly one install path to keep working rather than a second, updater-only
// code path that drifts out of sync with it.
func installScriptAsset() (asset string, interpreter string, args []string) {
	if runtime.GOOS == "windows" {
		return "install.ps1", "powershell", []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File"}
	}
	return "install.sh", "sh", nil
}

// maybeAutoUpdateCLI checks at most once a day for a newer published release
// and, when one exists, hands the upgrade to the platform install script in a
// detached process.
//
// It never blocks its caller: the version check is bounded by the context it
// is given, and the installer itself is started and abandoned rather than
// waited on, so the background agent's scan loop continues immediately. The
// running executable is replaced on disk by the script; this process keeps
// running the old code until it next restarts, which is why nothing here
// tries to re-exec.
//
// Signature verification is not skipped by going through the script: both
// install.sh and install.ps1 download the release signature alongside the
// binary and refuse to install if it does not verify. Reusing them keeps the
// unattended path exactly as strict as the manual one.
func maybeAutoUpdateCLI(ctx context.Context, rt runtimeContext) {
	current, err := rt.state.Load(ctx)
	if err != nil {
		return
	}
	if autoUpdateDisabled(struct{ Disabled bool }{current.AutoUpdateCLIDisabled}) {
		return
	}
	if !current.LastCLIUpdateCheck.IsZero() && time.Since(current.LastCLIUpdateCheck) < autoUpdateInterval {
		return
	}

	// Record the attempt before making the call, so a backend that is down, or
	// a machine that is offline all week, retries daily rather than on every
	// single agent tick.
	current.LastCLIUpdateCheck = time.Now().UTC()
	if saveErr := rt.state.Save(ctx, current); saveErr != nil {
		return
	}

	release, err := api.LatestGitHubRelease(ctx, proofboardGitHubRepo)
	if err != nil || release.TagName == "" {
		return
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	newer, err := dictionary.CompareVersions(latest, version.Version)
	if err != nil || newer <= 0 {
		return
	}

	if err := launchInstallScript(ctx, rt, release); err != nil {
		_ = logging.WriteSyncLog(rt.homeDir, "", "auto-update", "launch installer", "failure", err.Error())
		return
	}
	_ = logging.WriteSyncLog(rt.homeDir, "", "auto-update", "launch installer", "success",
		fmt.Sprintf("%s -> %s", version.Version, latest))
}

// launchInstallScript downloads the platform install script for this release
// and runs it detached. Output goes to the Career Agent's own log rather than
// a terminal: this runs unattended from the background agent, so there is no
// terminal to write to and a silent failure would otherwise be invisible.
func launchInstallScript(ctx context.Context, rt runtimeContext, release api.GitHubRelease) error {
	asset, interpreter, interpreterArgs := installScriptAsset()

	assetURL, ok := release.AssetURL(asset)
	if !ok {
		return fmt.Errorf("release %s has no %s asset", release.TagName, asset)
	}

	scriptDir, err := os.MkdirTemp("", "proofboard-update-")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	scriptPath := filepath.Join(scriptDir, asset)

	scriptFile, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		os.RemoveAll(scriptDir)
		return fmt.Errorf("create %s: %w", asset, err)
	}
	// Reuses the release client so the download inherits its HTTPS-only
	// enforcement and redirect validation rather than fetching the script
	// over a second, unchecked path.
	downloadErr := api.NewReleaseClient(rt.config.ReleaseBaseURL).Download(ctx, assetURL, scriptFile)
	scriptFile.Close()
	if downloadErr != nil {
		os.RemoveAll(scriptDir)
		return fmt.Errorf("download %s: %w", asset, downloadErr)
	}

	logPath := filepath.Join(rt.homeDir, ".proofboard", "auto-update.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		os.RemoveAll(scriptDir)
		return fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		os.RemoveAll(scriptDir)
		return fmt.Errorf("open update log: %w", err)
	}

	// Deliberately NOT bound to ctx: the caller's context belongs to the
	// agent's scan tick and is cancelled as soon as that tick returns, which
	// would kill the installer mid-download. The installer gets its own
	// timeout instead.
	installCtx, cancel := context.WithTimeout(context.Background(), installScriptTimeout)

	args := append(append([]string{}, interpreterArgs...), scriptPath)
	cmd := exec.CommandContext(installCtx, interpreter, args...)
	// Pin the installer to the release this updater actually decided on.
	// Without it the script re-resolves "latest" on its own and, if both of
	// its lookups fail, falls back to a version pinned inside the script —
	// which would silently DOWNGRADE a machine running a newer build,
	// unattended, and again every day after. The tag is already known here,
	// so there is no reason to let the script guess.
	cmd.Env = append(os.Environ(), "PROOFBOARD_VERSION="+release.TagName)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := startDetachedCommand(cmd); err != nil {
		cancel()
		logFile.Close()
		os.RemoveAll(scriptDir)
		return fmt.Errorf("start %s: %w", asset, err)
	}

	// Reaped in the background so the installer is never left a zombie and the
	// temporary script is always removed, without making the caller wait.
	go func() {
		defer cancel()
		defer logFile.Close()
		defer os.RemoveAll(scriptDir)
		_ = cmd.Wait()
	}()
	return nil
}
