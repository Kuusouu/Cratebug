//go:build windows

package reveal

import (
	"fmt"
	"os/exec"
)

func openPath(target Target) error {
	args := []string{target.Path}
	if target.SelectItem {
		// /select, and the path must be one argument; a separate "/select,"
		// argument inserts a space that Explorer does not treat as select.
		args = []string{"/select," + target.Path}
	}

	// explorer.exe exits with status 1 even when it opened the window, so Start
	// is the success signal; waiting on the process is not.
	if err := exec.Command("explorer.exe", args...).Start(); err != nil {
		return fmt.Errorf("open File Explorer: %w", err)
	}
	return nil
}
