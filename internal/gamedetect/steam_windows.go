//go:build windows

package gamedetect

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	steamRegistryKey   = `Software\Valve\Steam`
	steamRegistryValue = "SteamPath"
)

var defaultSteamFallbackRoots = []string{
	`C:\Program Files (x86)\Steam`,
	`C:\Program Files\Steam`,
	`D:\Steam`,
	`D:\Program Files (x86)\Steam`,
	`D:\Program Files\Steam`,
	`E:\Steam`,
	`E:\SteamLibrary`,
	`F:\Steam`,
	`F:\SteamLibrary`,
}

// defaultSteamPath reads Steam's install root from the Windows registry.
func defaultSteamPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, steamRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open the Steam registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(steamRegistryValue)
	if err != nil {
		return "", fmt.Errorf("read the %s registry value: %w", steamRegistryValue, err)
	}
	return filepath.FromSlash(value), nil
}
