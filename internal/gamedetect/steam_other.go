//go:build !windows && !linux

package gamedetect

var defaultSteamFallbackRoots []string

// defaultSteamPath returns an empty string on non-Windows/non-Linux platforms.
func defaultSteamPath() (string, error) {
	return "", nil
}
