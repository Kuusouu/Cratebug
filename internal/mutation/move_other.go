//go:build !windows

package mutation

import (
	"fmt"
	"os"
)

// moveFileWithoutReplace fails if the destination already exists, preserving the no-clobber guarantee.
func moveFileWithoutReplace(source, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination file already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check destination file: %w", err)
	}
	return os.Rename(source, destination)
}
