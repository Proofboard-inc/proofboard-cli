package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// installLocation is where the global Career Agent executable lives.
type installLocation struct {
	Dir        string
	Executable string
	SystemWide bool
}

// installEnvironment carries everything location resolution depends on, so the
// rules can be exercised without touching the machine running the tests.
type installEnvironment struct {
	GOOS         string
	HomeDir      string
	Euid         int
	Getenv       func(string) string
	FileWritable func(string) bool
}

func currentInstallEnvironment() (installEnvironment, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return installEnvironment{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return installEnvironment{
		GOOS:         runtime.GOOS,
		HomeDir:      homeDir,
		Euid:         os.Geteuid(),
		Getenv:       os.Getenv,
		FileWritable: fileWritable,
	}, nil
}

func fileWritable(path string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func executableName(goos string) string {
	if goos == "windows" {
		return "proofboard.exe"
	}
	return "proofboard"
}

func systemInstallDir(env installEnvironment) string {
	if env.GOOS == "windows" {
		programFiles := env.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		return filepath.Join(programFiles, "Proofboard")
	}
	return "/usr/local/bin"
}

func userInstallDir(env installEnvironment) string {
	if env.GOOS == "windows" {
		localAppData := env.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(env.HomeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "Programs", "Proofboard")
	}
	return filepath.Join(env.HomeDir, ".local", "bin")
}

// resolveInstallLocation picks where the executable is installed. Installing
// must not require administrator access, so a per-user directory is the
// default; a system-wide directory is used only when it is explicitly asked
// for, when the process already has the rights for it, or when a system-wide
// installation is already present and writable.
func resolveInstallLocation(env installEnvironment, requestSystemWide bool) installLocation {
	name := executableName(env.GOOS)

	if override := env.Getenv("PROOFBOARD_INSTALL_DIR"); override != "" {
		return installLocation{Dir: override, Executable: filepath.Join(override, name), SystemWide: false}
	}

	systemDir := systemInstallDir(env)
	systemExecutable := filepath.Join(systemDir, name)

	if requestSystemWide {
		return installLocation{Dir: systemDir, Executable: systemExecutable, SystemWide: true}
	}
	if env.GOOS != "windows" && env.Euid == 0 {
		return installLocation{Dir: systemDir, Executable: systemExecutable, SystemWide: true}
	}
	// Keep upgrading an existing system-wide installation in place whenever
	// that is possible without extra rights.
	if env.FileWritable != nil && env.FileWritable(systemExecutable) {
		return installLocation{Dir: systemDir, Executable: systemExecutable, SystemWide: true}
	}

	userDir := userInstallDir(env)
	return installLocation{Dir: userDir, Executable: filepath.Join(userDir, name), SystemWide: false}
}

// knownInstallLocations lists every place an executable may have been
// installed, so uninstall can clean up an installation made by an earlier
// version or by a system-wide installer package.
func knownInstallLocations(env installEnvironment) []installLocation {
	name := executableName(env.GOOS)
	seen := map[string]bool{}
	locations := []installLocation{}

	add := func(dir string, systemWide bool) {
		if dir == "" {
			return
		}
		executable := filepath.Join(dir, name)
		if seen[executable] {
			return
		}
		seen[executable] = true
		locations = append(locations, installLocation{Dir: dir, Executable: executable, SystemWide: systemWide})
	}

	add(env.Getenv("PROOFBOARD_INSTALL_DIR"), false)
	add(userInstallDir(env), false)
	add(systemInstallDir(env), true)
	return locations
}

func pathContainsDir(pathValue, dir string) bool {
	if dir == "" {
		return false
	}
	cleaned := filepath.Clean(dir)
	for _, entry := range filepath.SplitList(pathValue) {
		if entry == "" {
			continue
		}
		if filepath.Clean(entry) == cleaned {
			return true
		}
		if runtime.GOOS == "windows" && strings.EqualFold(filepath.Clean(entry), cleaned) {
			return true
		}
	}
	return false
}
