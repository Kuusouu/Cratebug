//go:build linux

package gamedetect

import (
	"path/filepath"
	"testing"
)

func TestLinuxSteamFallbackRootsContainsExpectedPaths(t *testing.T) {
	// Arrange
	home := "/home/testuser"

	// Act
	roots := linuxSteamFallbackRoots(home)

	// Assert
	expectedRoots := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".steam", "debian-installation"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam", "steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam", "root"),
	}

	for _, expected := range expectedRoots {
		found := false
		for _, root := range roots {
			if root == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("linuxSteamFallbackRoots() missing expected root %q", expected)
		}
	}
}
