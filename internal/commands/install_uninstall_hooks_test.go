package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/model"
)

func TestRemoveAllHeaderBlocksRemovesEveryOccurrence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := strings.Join([]string{
		"export EDITOR=vim",
		"",
		workspaceDetectionHeader,
		`[[ -n "$PROOFBOARD_SKIP" ]] || proofboard detect 2>/dev/null`,
		"",
		workspaceDetectionHeader,
		`proofboard notices 2>/dev/null`,
		"",
		"alias ll='ls -la'",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}

	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	result := string(got)
	if strings.Contains(result, workspaceDetectionHeader) {
		t.Errorf("header still present: %q", result)
	}
	if strings.Contains(result, "proofboard detect") || strings.Contains(result, "proofboard notices") {
		t.Errorf("hook lines still present: %q", result)
	}
	if !strings.Contains(result, "export EDITOR=vim") || !strings.Contains(result, "alias ll='ls -la'") {
		t.Errorf("unrelated lines were dropped: %q", result)
	}
}

// TestRemoveAllHeaderBlocksRemovesMultiLineHookIntact is a regression test
// for a real incident: removeAllHeaderBlocks used to strip only the header
// plus exactly one following line. zshChpwdHook (shell_hooks.go) is several
// lines wrapped in "if [ -z \"$PROOFBOARD_CHPWD_INSTALLED\" ]; then ... fi" —
// the old code stripped the header and the opening "if ... then" line but
// left the function body and the closing "fi" behind, orphaned with no
// matching "if". That produced a fatal `zsh: parse error near 'fi'` on every
// new shell after running `proofboard uninstall`, which also silently broke
// the workspace-detection hook in every terminal (VS Code's integrated
// terminal included) since zsh never finished sourcing .zshrc. Blocks are
// blank-line-delimited by construction (ensureLineInFile's "\n%s\n%s\n"
// writes), so removal must read to the next blank line, not a fixed count.
func TestRemoveAllHeaderBlocksRemovesMultiLineHookIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := strings.Join([]string{
		`export NVM_DIR="$HOME/.nvm"`,
		"",
		workspaceDetectionHeader,
		zshChpwdHook(defaultHookCommand),
		"",
		autocompletionHeader,
		"source <(proofboard completion zsh)",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}

	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}
	if err := removeAllHeaderBlocks(path, autocompletionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	result := string(got)
	if strings.Contains(result, "PROOFBOARD_CHPWD_INSTALLED") ||
		strings.Contains(result, "_proofboard_chpwd") ||
		strings.Contains(result, "add-zsh-hook") {
		t.Fatalf("chpwd hook body was not fully removed: %q", result)
	}
	// The most direct regression check: no orphaned "fi" with no matching
	// "if" left anywhere in the file.
	ifCount := strings.Count(result, "if ")
	fiCount := 0
	for _, line := range strings.Split(result, "\n") {
		if strings.TrimSpace(line) == "fi" {
			fiCount++
		}
	}
	if fiCount > ifCount {
		t.Fatalf("orphaned 'fi' with no matching 'if' left behind: %q", result)
	}
	if !strings.Contains(result, `export NVM_DIR="$HOME/.nvm"`) {
		t.Errorf("unrelated line was dropped: %q", result)
	}
}

func TestRemoveAllHeaderBlocksNoopWhenHeaderAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := "export EDITOR=vim\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}
	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	if string(got) != content {
		t.Errorf("file changed when header was absent: %q", string(got))
	}
}

func TestRemoveAllHeaderBlocksNoopWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks on missing file: %v", err)
	}
}

// FIX: performUninstall previously only stripped the PATH export header
// (proofboardPathHeader) via removeDirectoryFromPath, leaving the
// workspace-detection and autocompletion header blocks behind in every rc
// file they were written to. removeShellHookBlocks must strip all three.
func TestRemoveShellHookBlocksStripsAllThreeHeadersFromEveryRcFile(t *testing.T) {
	homeDir := t.TempDir()
	env := installEnvironment{GOOS: "linux", HomeDir: homeDir}

	zshrc := filepath.Join(homeDir, ".zshrc")
	zshrcContent := strings.Join([]string{
		proofboardPathHeader,
		`export PATH="/home/engineer/.local/bin:$PATH"`,
		"",
		workspaceDetectionHeader,
		"proofboard detect 2>/dev/null",
		"",
		autocompletionHeader,
		"source <(proofboard completion zsh)",
		"",
		"# user's own config below",
		"alias gs='git status'",
	}, "\n")
	if err := os.WriteFile(zshrc, []byte(zshrcContent), 0o644); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	zprofile := filepath.Join(homeDir, ".zprofile")
	zprofileContent := strings.Join([]string{
		proofboardPathHeader,
		`export PATH="/home/engineer/.local/bin:$PATH"`,
	}, "\n")
	if err := os.WriteFile(zprofile, []byte(zprofileContent), 0o644); err != nil {
		t.Fatalf("write .zprofile: %v", err)
	}

	removeShellHookBlocks(env)

	zshrcGot, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	for _, header := range []string{proofboardPathHeader, workspaceDetectionHeader, autocompletionHeader} {
		if strings.Contains(string(zshrcGot), header) {
			t.Errorf(".zshrc still contains header %q: %q", header, zshrcGot)
		}
	}
	if !strings.Contains(string(zshrcGot), "alias gs='git status'") {
		t.Errorf(".zshrc lost unrelated content: %q", zshrcGot)
	}

	zprofileGot, err := os.ReadFile(zprofile)
	if err != nil {
		t.Fatalf("read .zprofile: %v", err)
	}
	if strings.Contains(string(zprofileGot), proofboardPathHeader) {
		t.Errorf(".zprofile still contains PATH header: %q", zprofileGot)
	}
}

func TestPerformUninstallStripsShellHookBlocksEvenWhenNoExecutableFound(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	t.Setenv("PROOFBOARD_INSTALL_DIR", "")

	zshrc := filepath.Join(homeDir, ".zshrc")
	content := strings.Join([]string{
		workspaceDetectionHeader,
		"proofboard detect 2>/dev/null",
		autocompletionHeader,
		"source <(proofboard completion zsh)",
	}, "\n")
	if err := os.WriteFile(zshrc, []byte(content), 0o644); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	var out strings.Builder
	if err := performUninstall(context.Background(), &out); err != nil {
		t.Fatalf("performUninstall: %v", err)
	}

	got, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if strings.Contains(string(got), workspaceDetectionHeader) || strings.Contains(string(got), autocompletionHeader) {
		t.Errorf("shell hook blocks remained after uninstall with no executable found: %q", got)
	}
}

// FIX: performUninstall previously removed the executable, the background
// service, and shell hooks, but never touched the stored session
// credentials or device signing key. A device that had just been
// "uninstalled" was still fully authenticated — reinstalling reported the
// same account as already connected. Uninstall must sign the device out,
// same as `proofboard logout`.
func TestPerformUninstallClearsCredentialsAndDeviceKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PROOFBOARD_INSTALL_DIR", "")
	t.Setenv("PROOFBOARD_DISABLE_KEYCHAIN", "1")

	if err := pbauth.NewCredentialStore(homeDir).Save(context.Background(),
		model.Credentials{Token: "test-token", EmailHash: "hash"}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	if err := pbauth.NewDeviceKeyStore(homeDir).Save(context.Background(), pbauth.DeviceKeyRecord{
		Algorithm:  "ECDSA_P256_SHA256",
		PublicKey:  "public",
		PrivateKey: "private",
	}); err != nil {
		t.Fatalf("seed device key: %v", err)
	}

	var out strings.Builder
	if err := performUninstall(context.Background(), &out); err != nil {
		t.Fatalf("performUninstall: %v", err)
	}

	if _, err := os.Stat(filepath.Join(homeDir, ".proofboard", "credentials.json")); !os.IsNotExist(err) {
		t.Errorf("uninstall left the credentials file in place")
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".proofboard", "device.key")); !os.IsNotExist(err) {
		t.Errorf("uninstall left the device key file in place")
	}
	if !strings.Contains(out.String(), "Signed out") {
		t.Errorf("uninstall did not report signing out: %q", out.String())
	}
}

// FIX: `proofboard uninstall` used to remove everything with no confirmation
// at all — destructive, and impossible to script safely without risking an
// accidental run. It must ask first, and --yes must skip that prompt for
// scripted/non-interactive use.
func TestUninstallCommandPromptsForConfirmation(t *testing.T) {
	t.Run("declining leaves the action uncalled", func(t *testing.T) {
		calls := 0
		cmd := newUninstallCommandWithAction(context.Background(), func(ctx context.Context, out io.Writer) error {
			calls++
			return nil
		})
		var out strings.Builder
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader("n\n"))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("uninstall command: %v", err)
		}
		if calls != 0 {
			t.Fatalf("declining the prompt should not run uninstall, got %d calls", calls)
		}
		if !strings.Contains(out.String(), "cancelled") {
			t.Errorf("expected a cancellation message, got %q", out.String())
		}
	})

	t.Run("confirming runs the action", func(t *testing.T) {
		calls := 0
		cmd := newUninstallCommandWithAction(context.Background(), func(ctx context.Context, out io.Writer) error {
			calls++
			return nil
		})
		var out strings.Builder
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader("y\n"))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("uninstall command: %v", err)
		}
		if calls != 1 {
			t.Fatalf("confirming the prompt should run uninstall once, got %d calls", calls)
		}
	})

	t.Run("--yes skips the prompt entirely", func(t *testing.T) {
		calls := 0
		cmd := newUninstallCommandWithAction(context.Background(), func(ctx context.Context, out io.Writer) error {
			calls++
			return nil
		})
		var out strings.Builder
		cmd.SetOut(&out)
		// No stdin wired up at all — if the command tried to read a
		// confirmation it would block or error, not silently succeed.
		cmd.SetIn(strings.NewReader(""))
		cmd.SetArgs([]string{"--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("uninstall command: %v", err)
		}
		if calls != 1 {
			t.Fatalf("--yes should run uninstall once without a prompt, got %d calls", calls)
		}
		if strings.Contains(out.String(), "Continue?") {
			t.Errorf("--yes should skip the confirmation prompt, got %q", out.String())
		}
	})

	t.Run("no input available defaults to not proceeding", func(t *testing.T) {
		calls := 0
		cmd := newUninstallCommandWithAction(context.Background(), func(ctx context.Context, out io.Writer) error {
			calls++
			return nil
		})
		var out strings.Builder
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader(""))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("uninstall command: %v", err)
		}
		if calls != 0 {
			t.Fatalf("empty stdin should default to declining, got %d calls", calls)
		}
	})
}
