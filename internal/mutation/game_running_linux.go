//go:build linux

package mutation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Default system procfs path on Linux.
	defaultProcDir = "/proc"

	// 15-character truncated process name assigned by Wine to marvel-win64-shipping.exe.
	wineTruncatedComm = "marvel-win64-sh"
)

// LinuxGameRunningChecker detects whether Marvel Rivals is running by inspecting /proc.
type LinuxGameRunningChecker struct {
	procDir string
}

// NewGameRunningChecker creates the default Linux game-running detector.
func NewGameRunningChecker() GameRunningChecker {
	return NewLinuxGameRunningChecker()
}

// NewLinuxGameRunningChecker creates a checker that inspects the system /proc directory.
func NewLinuxGameRunningChecker() LinuxGameRunningChecker {
	return LinuxGameRunningChecker{procDir: defaultProcDir}
}

// NewLinuxGameRunningCheckerForProc creates a checker inspecting a custom proc directory for tests.
func NewLinuxGameRunningCheckerForProc(procDir string) LinuxGameRunningChecker {
	return LinuxGameRunningChecker{procDir: procDir}
}

// IsGameRunning checks /proc for Marvel Rivals processes running natively or via Proton/Wine.
func (c LinuxGameRunningChecker) IsGameRunning() (bool, error) {
	procDir := c.procDir
	if procDir == "" {
		procDir = defaultProcDir
	}

	entries, err := os.ReadDir(procDir)
	if err != nil {
		return false, fmt.Errorf("read proc directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isNumericPID(entry.Name()) {
			continue
		}

		pidDir := filepath.Join(procDir, entry.Name())
		cmdlineBytes, err := os.ReadFile(filepath.Join(pidDir, "cmdline"))
		if err != nil {
			// Process may have exited or is unreadable.
			continue
		}

		commBytes, _ := os.ReadFile(filepath.Join(pidDir, "comm"))
		comm := strings.TrimSpace(string(commBytes))

		if isMarvelRivalsLinuxProcess(comm, cmdlineBytes) {
			return true, nil
		}
	}

	return false, nil
}

func isNumericPID(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isMarvelRivalsLinuxProcess(comm string, cmdlineBytes []byte) bool {
	// Wine truncates the comm name to 15 characters, yielding wineTruncatedComm ("marvel-win64-sh").
	isCommMatch := strings.EqualFold(comm, wineTruncatedComm) || isMarvelRivalsProcess(comm)

	rawTokens := bytes.Split(cmdlineBytes, []byte{0})
	var tokens []string
	for _, token := range rawTokens {
		cleaned := strings.TrimSpace(string(token))
		cleaned = strings.Trim(cleaned, `"'`)
		if cleaned != "" {
			tokens = append(tokens, cleaned)
		}
	}

	if len(tokens) == 0 {
		return isCommMatch
	}

	// Case 1: The executable is the direct command (arg 0).
	if isMarvelRivalsPath(tokens[0]) {
		return true
	}

	// Case 2: Running under a Wine / Proton loader (wine64-preloader, wine, etc.).
	if isWineLoader(tokens[0]) {
		for _, arg := range tokens[1:] {
			if strings.HasPrefix(arg, "-") || (strings.HasPrefix(arg, "/") && !strings.Contains(arg, `\`) && !strings.Contains(arg, ".exe")) {
				continue
			}
			if isMarvelRivalsPath(arg) {
				return true
			}
		}
	}

	// Case 3: Comm matched the Wine-truncated process name, and an argument references the executable.
	if isCommMatch {
		for _, arg := range tokens {
			if isMarvelRivalsPath(arg) {
				return true
			}
		}
		// If comm matches exactly, consider it running even without full cmdline.
		return true
	}

	return false
}

func isMarvelRivalsPath(path string) bool {
	normalized := strings.ReplaceAll(path, `\`, `/`)
	base := filepath.Base(normalized)
	return isMarvelRivalsProcess(base)
}

func isWineLoader(path string) bool {
	normalized := strings.ReplaceAll(path, `\`, `/`)
	base := strings.ToLower(filepath.Base(normalized))
	base = strings.TrimSuffix(base, ".exe")
	switch base {
	case "wine", "wine64", "wine-preloader", "wine64-preloader", "wineserver", "proton":
		return true
	default:
		return false
	}
}
