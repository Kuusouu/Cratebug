package mutation

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const marvelRivalsExecutable = "marvel-win64-shipping.exe"

// Detects Marvel Rivals by its established shipping executable name.
type WindowsGameRunningChecker struct{}

// Snapshots Windows processes so the answer is independent of Steam and Epic install paths.
func (WindowsGameRunningChecker) IsGameRunning() (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var process windows.ProcessEntry32
	process.Size = uint32(unsafe.Sizeof(process))
	if err := windows.Process32First(snapshot, &process); err != nil {
		return false, fmt.Errorf("read first process: %w", err)
	}

	for {
		if isMarvelRivalsProcess(windows.UTF16ToString(process.ExeFile[:])) {
			return true, nil
		}

		if err := windows.Process32Next(snapshot, &process); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return false, nil
			}
			return false, fmt.Errorf("read next process: %w", err)
		}
	}
}

// Keeps the established executable identity easy to test without a live process.
func isMarvelRivalsProcess(processName string) bool {
	return strings.EqualFold(processName, marvelRivalsExecutable)
}
