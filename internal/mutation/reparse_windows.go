//go:build windows

package mutation

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func verifyNotReparsePoint(path, label string) error {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode %s path: %w", label, err)
	}

	attributes, err := windows.GetFileAttributes(pathPointer)
	if err != nil {
		return fmt.Errorf("read %s attributes: %w", label, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%s is a reparse-point directory: %q", label, path)
	}
	return nil
}
