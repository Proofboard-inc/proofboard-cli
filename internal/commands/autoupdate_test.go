package commands

import (
	"runtime"
	"strings"
	"testing"
)

func TestAutoUpdateDisabledHonoursEnvAndState(t *testing.T) {
	if autoUpdateDisabled(struct{ Disabled bool }{false}) {
		t.Fatal("auto-update must be ON by default: a zero-value state.json written before the field existed must not silently opt a machine out")
	}
	if !autoUpdateDisabled(struct{ Disabled bool }{true}) {
		t.Fatal("the persisted opt-out was ignored")
	}
	t.Setenv("PROOFBOARD_DISABLE_AUTO_UPDATE", "1")
	if !autoUpdateDisabled(struct{ Disabled bool }{false}) {
		t.Fatal("PROOFBOARD_DISABLE_AUTO_UPDATE=1 must override the persisted default")
	}
}

// The updater must reuse the same install scripts a first-time user runs, so
// there is one install path to keep working rather than an updater-only one
// that drifts. Those scripts verify the release signature before installing,
// which is what keeps the unattended path as strict as the manual one.
func TestInstallScriptAssetMatchesPlatform(t *testing.T) {
	asset, interpreter, args := installScriptAsset()
	if runtime.GOOS == "windows" {
		if asset != "install.ps1" || interpreter != "powershell" {
			t.Fatalf("windows should install via install.ps1/powershell, got %s/%s", asset, interpreter)
		}
		if !strings.Contains(strings.Join(args, " "), "-NoProfile") {
			t.Fatalf("powershell must run with -NoProfile so a user profile cannot alter an unattended install, got %v", args)
		}
		return
	}
	if asset != "install.sh" || interpreter != "sh" {
		t.Fatalf("posix should install via install.sh/sh, got %s/%s", asset, interpreter)
	}
}

func TestAutoUpdateIntervalIsDaily(t *testing.T) {
	if autoUpdateInterval.Hours() != 24 {
		t.Fatalf("auto-update interval = %v, want 24h", autoUpdateInterval)
	}
}
