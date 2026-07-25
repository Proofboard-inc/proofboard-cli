package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testInstallEnvironment(goos, homeDir string, values map[string]string) installEnvironment {
	return installEnvironment{
		GOOS:    goos,
		HomeDir: homeDir,
		Euid:    1000,
		Getenv: func(key string) string {
			return values[key]
		},
		FileWritable: func(string) bool { return false },
	}
}

func TestResolveInstallLocationDefaultsToUserDirectory(t *testing.T) {
	env := testInstallEnvironment("linux", "/home/engineer", nil)

	location := resolveInstallLocation(env, false)

	if location.SystemWide {
		t.Fatalf("install without administrator access must not be system wide: %+v", location)
	}
	if location.Executable != "/home/engineer/.local/bin/proofboard" {
		t.Fatalf("unexpected executable location: %s", location.Executable)
	}
}

func TestResolveInstallLocationDefaultsToUserDirectoryOnWindows(t *testing.T) {
	env := testInstallEnvironment("windows", `C:\Users\engineer`, map[string]string{
		"LOCALAPPDATA": `C:\Users\engineer\AppData\Local`,
		"ProgramFiles": `C:\Program Files`,
	})

	location := resolveInstallLocation(env, false)

	if location.SystemWide {
		t.Fatalf("install without administrator access must not be system wide: %+v", location)
	}
	expected := filepath.Join(`C:\Users\engineer\AppData\Local`, "Programs", "Proofboard", "proofboard.exe")
	if location.Executable != expected {
		t.Fatalf("unexpected executable location: %s", location.Executable)
	}
}

func TestResolveInstallLocationHonoursExplicitRequests(t *testing.T) {
	env := testInstallEnvironment("linux", "/home/engineer", nil)

	location := resolveInstallLocation(env, true)

	if !location.SystemWide || location.Executable != "/usr/local/bin/proofboard" {
		t.Fatalf("--system must install system wide: %+v", location)
	}

	override := testInstallEnvironment("linux", "/home/engineer", map[string]string{
		"PROOFBOARD_INSTALL_DIR": "/opt/proofboard/bin",
	})
	location = resolveInstallLocation(override, false)
	if location.SystemWide || location.Executable != "/opt/proofboard/bin/proofboard" {
		t.Fatalf("explicit install directory was ignored: %+v", location)
	}
}

func TestResolveInstallLocationKeepsExistingSystemInstallation(t *testing.T) {
	env := testInstallEnvironment("linux", "/home/engineer", nil)
	env.FileWritable = func(path string) bool { return path == "/usr/local/bin/proofboard" }

	location := resolveInstallLocation(env, false)

	if !location.SystemWide || location.Executable != "/usr/local/bin/proofboard" {
		t.Fatalf("a writable system installation should be upgraded in place: %+v", location)
	}

	root := testInstallEnvironment("linux", "/root", nil)
	root.Euid = 0
	if location = resolveInstallLocation(root, false); !location.SystemWide {
		t.Fatalf("root should install system wide: %+v", location)
	}
}

func TestKnownInstallLocationsCoverUserAndSystemDirectories(t *testing.T) {
	env := testInstallEnvironment("linux", "/home/engineer", nil)

	locations := knownInstallLocations(env)

	found := map[string]bool{}
	for _, location := range locations {
		found[location.Executable] = true
	}
	for _, expected := range []string{"/home/engineer/.local/bin/proofboard", "/usr/local/bin/proofboard"} {
		if !found[expected] {
			t.Fatalf("uninstall would miss %s: %+v", expected, locations)
		}
	}
}

func TestPathContainsDir(t *testing.T) {
	pathValue := strings.Join([]string{"/usr/bin", "/home/engineer/.local/bin/", "/bin"}, string(os.PathListSeparator))

	if !pathContainsDir(pathValue, "/home/engineer/.local/bin") {
		t.Fatal("directory already on PATH was not detected")
	}
	if pathContainsDir(pathValue, "/opt/proofboard/bin") {
		t.Fatal("directory absent from PATH was reported as present")
	}
}

func TestInstallToUsesRequestedDirectoryWithoutElevation(t *testing.T) {
	if os.Getenv("PROOFBOARD_INSTALL_TARGET_CHILD") == "1" {
		return
	}
	installDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("PROOFBOARD_INSTALL_DIR", installDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", installDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	env, err := currentInstallEnvironment()
	if err != nil {
		t.Fatalf("resolve install environment: %v", err)
	}
	location := resolveInstallLocation(env, false)
	if location.Dir != installDir {
		t.Fatalf("install directory override was ignored: %s", location.Dir)
	}

	source := filepath.Join(t.TempDir(), "proofboard-source")
	if err := os.WriteFile(source, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write source executable: %v", err)
	}

	builder := &strings.Builder{}
	if err := copyExecutable(source, location, builder); err != nil {
		t.Fatalf("copy executable: %v", err)
	}
	info, err := os.Stat(location.Executable)
	if err != nil {
		t.Fatalf("installed executable is missing: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed executable is not executable: %v", info.Mode().Perm())
	}
	if _, err := os.Stat(location.Executable + ".new"); !os.IsNotExist(err) {
		t.Fatalf("staged executable was left behind: %v", err)
	}
}

func TestEnsureDirectoryOnPathUpdatesShellProfile(t *testing.T) {
	homeDir := t.TempDir()
	installDir := filepath.Join(homeDir, ".local", "bin")
	env := installEnvironment{
		GOOS:    "linux",
		HomeDir: homeDir,
		Euid:    1000,
		Getenv: func(key string) string {
			switch key {
			case "SHELL":
				return "/bin/bash"
			case "PATH":
				return "/usr/bin:/bin"
			}
			return ""
		},
		FileWritable: func(string) bool { return false },
	}

	builder := &strings.Builder{}
	if err := ensureDirectoryOnPath(env, installDir, builder); err != nil {
		t.Fatalf("ensure directory on PATH: %v", err)
	}

	profile := filepath.Join(homeDir, ".bashrc")
	content, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if !strings.Contains(string(content), installDir) {
		t.Fatalf("PATH entry was not added: %s", content)
	}

	// A second install must not append the entry again.
	before := string(content)
	if err := ensureDirectoryOnPath(env, installDir, builder); err != nil {
		t.Fatalf("second ensure directory on PATH: %v", err)
	}
	content, err = os.ReadFile(profile)
	if err != nil {
		t.Fatalf("re-read profile: %v", err)
	}
	if string(content) != before {
		t.Fatalf("PATH entry was added twice:\n%s", content)
	}

	// Uninstall removes it again.
	removeDirectoryFromPath(env, installDir)
	content, err = os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile after removal: %v", err)
	}
	if strings.Contains(string(content), installDir) {
		t.Fatalf("PATH entry remained after uninstall: %s", content)
	}
}

func TestEnsureDirectoryOnPathSkipsDirectoriesAlreadyOnPath(t *testing.T) {
	homeDir := t.TempDir()
	installDir := filepath.Join(homeDir, ".local", "bin")
	env := installEnvironment{
		GOOS:    "linux",
		HomeDir: homeDir,
		Euid:    1000,
		Getenv: func(key string) string {
			switch key {
			case "SHELL":
				return "/bin/bash"
			case "PATH":
				return installDir + string(os.PathListSeparator) + "/usr/bin"
			}
			return ""
		},
		FileWritable: func(string) bool { return false },
	}

	if err := ensureDirectoryOnPath(env, installDir, &strings.Builder{}); err != nil {
		t.Fatalf("ensure directory on PATH: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".bashrc")); !os.IsNotExist(err) {
		t.Fatalf("shell profile was modified needlessly: %v", err)
	}
}
