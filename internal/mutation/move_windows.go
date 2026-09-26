//go:build windows

package mutation

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// moveFileWithoutReplace uses MoveFileW because it fails if the destination exists.
// os.Rename does not provide that no-clobber guarantee on Windows.
func moveFileWithoutReplace(source, destination string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encode source path: %w", err)
	}

	destinationPointer, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return fmt.Errorf("encode destination path: %w", err)
	}

	return windows.MoveFile(sourcePointer, destinationPointer)
}
