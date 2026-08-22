package mutation

import (
	"fmt"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellDelete              = 3
	shellAllowUndo           = 0x0040
	shellNoConfirmation      = 0x0010
	shellSilent              = 0x0004
	shellWarnBeforePermanent = 0x4000
)

type shellFileOperation struct {
	windowHandle  uintptr
	operation     uint32
	from          *uint16
	to            *uint16
	flags         uint16
	aborted       int32
	nameMappings  uintptr
	progressTitle *uint16
}

var shell32 = windows.NewLazySystemDLL("shell32.dll")
var shellFileOperationProc = shell32.NewProc("SHFileOperationW")

// Sends every path to the Windows Recycle Bin as one shell operation. The
// warning flag prevents an unavailable Recycle Bin from becoming silent data loss.
func recycleFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("Recycle Bin operation has no paths")
	}

	// SHFileOperationW accepts a double-null-terminated list, whereas
	// UTF16FromString intentionally rejects embedded NUL separators.
	from := utf16.Encode([]rune(joinShellPaths(paths)))
	operation := shellFileOperation{
		operation: shellDelete,
		from:      &from[0],
		flags:     shellAllowUndo | shellNoConfirmation | shellSilent | shellWarnBeforePermanent,
	}
	result, _, callErr := shellFileOperationProc.Call(uintptr(unsafe.Pointer(&operation)))
	runtime.KeepAlive(from)
	if result != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("Windows shell deletion failed with code %d: %w", result, callErr)
		}
		return fmt.Errorf("Windows shell deletion failed with code %d", result)
	}
	if operation.aborted != 0 {
		return fmt.Errorf("Windows shell deletion was aborted")
	}
	return nil
}

func joinShellPaths(paths []string) string {
	result := ""
	for _, path := range paths {
		result += path + "\x00"
	}
	return result + "\x00"
}
