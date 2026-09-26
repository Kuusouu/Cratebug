//go:build !windows

package update

import "fmt"

// ApplyUpdate is only supported on Windows.
func ApplyUpdate(installerPath string) error {
	return fmt.Errorf("update: automatic updater is only supported on Windows")
}
