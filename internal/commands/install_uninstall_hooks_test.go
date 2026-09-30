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

// Uninstall must remove the block this CLI wrote and nothing else. A user who
// appends their own configuration to a rc file right after installing
// Proofboard — `cat >> ~/.zshrc`, an editor that saves straight to the end of
// the buffer — leaves no blank line between the block and their line, which is
// exactly the shape that a "read to the next blank line" sweep silently eats.
// The hook here is written against an absolute binary path (the PATH
// -independent form install actually writes) so this also covers matching a
// body whose path differs from the current process's.
func TestRemoveAllHeaderBlocksKeepsUserLinesAppendedDirectlyAfterABlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := strings.Join([]string{
		workspaceDetectionHeader,
		zshChpwdHook("/opt/proofboard/bin/proofboard"),
		`alias pbdiff="git diff --stat"`,
		"",
		autocompletionHeader,
		"source <(/opt/proofboard/bin/proofboard completion zsh)",
		`export EDITOR=nvim`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rc file: %v", err)
	}

	if err := removeAllHeaderBlocks(path, workspaceDetectionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks(workspace detection): %v", err)
	}
	if err := removeAllHeaderBlocks(path, autocompletionHeader); err != nil {
		t.Fatalf("removeAllHeaderBlocks(autocompletion): %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	result := string(got)
	for _, removed := range []string{workspaceDetectionHeader, autocompletionHeader, "PROOFBOARD_CHPWD_INSTALLED", "add-zsh-hook", "completion zsh", "/opt/proofboard"} {
		if strings.Contains(result, removed) {
			t.Errorf("%q survived removal: %q", removed, result)
		}
	}
	if !strings.Contains(result, `alias pbdiff="git diff --stat"`) {
		t.Errorf("a user line written directly after a block was deleted: %q", result)
	}
	if !strings.Contains(result, `export EDITOR=nvim`) {
		t.Errorf("a user line written directly after a block was deleted: %q", result)
	}
}

// The legacy fish backgrounded body is two lines under one header, and the
// older PowerShell body is a single Start-Process line. Both predate the
// current templates but are still recognised by ensureLineInFile's migration
// path, so uninstall has to recognise them too.
func TestRemoveAllHeaderBlocksRemovesLegacyBodies(t *testing.T) {
	for name, body := range map[string]string{
		"fish backgrounded": legacyFishBackgroundedDetectionLine,
		"powershell hidden": legacyPSDetectionLine,
		"plain posix":       legacyPlainShellDetectionLine,
		"backgrounded":      legacyBackgroundedShellDetectionLine,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".profile")
			content := strings.Join([]string{
				`export EDITOR=vim`,
				workspaceDetectionHeader,
				body,
				`alias ll='ls -la'`,
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
				t.Errorf("header survived: %q", result)
			}
			for _, line := range strings.Split(body, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if strings.Contains(result, strings.TrimSpace(line)) {
					t.Errorf("legacy body line %q survived: %q", line, result)
				}
			}
			if !strings.Contains(result, "export EDITOR=vim") || !strings.Contains(result, "alias ll='ls -la'") {
				t.Errorf("unrelated lines were dropped: %q", result)
			}
		})
	}
}

// The PATH block is written by install, not by the hook templates, and comes
// in a fish flavour as well as the POSIX export. Both must be recognised so
// uninstall leaves no reference to a removed binary — and the user's own PATH
// line sitting right below one must survive.
func TestRemoveAllHeaderBlocksRemovesPathBlocksOnly(t *testing.T) {
	cases := map[string]string{
		"export PATH=\"$HOME/.local/bin/proofboard\":$PATH": "",
		"fish_add_path \"$HOME/.local/bin/proofboard\"":     "",
	}
	for body := range cases {
		path := filepath.Join(t.TempDir(), ".profile")
		content := strings.Join([]string{
			proofboardPathHeader,
			body,
			`export PATH="$HOME/go/bin:$PATH"`,
		}, "\n")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write rc file: %v", err)
		}
		if err := removeAllHeaderBlocks(path, proofboardPathHeader); err != nil {
			t.Fatalf("removeAllHeaderBlocks: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read rc file: %v", err)
		}
		result := string(got)
		if strings.Contains(result, proofboardPathHeader) || strings.Contains(result, ".local/bin/proofboard") {
			t.Errorf("PATH block survived: %q", result)
		}
		if !strings.Contains(result, `export PATH="$HOME/go/bin:$PATH"`) {
			t.Errorf("the user's own PATH line was dropped: %q", result)
		}
	}
}

// A block this build has no template for — hand-edited, or left by a version
// whose constants are gone — must still be cleaned up as far as it can be,
// without reaching past the lines that name Proofboard into the user's own
// configuration, and without leaving a bare "fi" that would break the shell.
func TestRemoveAllHeaderBlocksHandlesUnknownBodyConservatively(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	content := strings.Join([]string{
		workspaceDetectionHeader,
		`if [ -z "$PROOFBOARD_CHPWD_INSTALLED" ]; then`,
		`  proofboard detect 2>/dev/null`,
		`fi`,
		`export SPACESHIP_MODE="rocket"`,
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
	if strings.Contains(result, "PROOFBOARD_CHPWD_INSTALLED") || strings.Contains(result, "proofboard detect") {
		t.Errorf("unknown block body survived: %q", result)
	}
	if !strings.Contains(result, `export SPACESHIP_MODE="rocket"`) {
		t.Errorf("a user line after an unknown block was deleted: %q", result)
	}
	// "fi" alone is a syntax error in a shell profile, so the sweep must
	// consume the closer it left behind rather than hand the user a file that
	// no longer parses.
	for _, line := range strings.Split(result, "\n") {
		if strings.TrimSpace(line) == "fi" {
			t.Errorf("orphaned 'fi' left behind: %q", result)
		}
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
