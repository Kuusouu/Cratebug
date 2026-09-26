//go:build linux

package gamedetect

import (
	"os"
	"path/filepath"
)

func linuxSteamFallbackRoots(homeDir string) []string {
	if homeDir == "" {
		return nil
	}
	return []string{
		filepath.Join(homeDir, ".local", "share", "Steam"),
		filepath.Join(homeDir, ".steam", "steam"),
		filepath.Join(homeDir, ".steam", "root"),
		filepath.Join(homeDir, ".steam", "debian-installation"),
		filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
		filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", ".steam", "steam"),
		filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", ".steam", "root"),
	}
}

var defaultSteamFallbackRoots = func() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return linuxSteamFallbackRoots(home)
}()

// defaultSteamPath returns an empty string on Linux because Steam libraries are found via standard fallback paths.
func defaultSteamPath() (string, error) {
	return "", nil
}
