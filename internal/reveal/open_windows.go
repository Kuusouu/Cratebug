//go:build windows

package reveal

import (
	"fmt"
	"os/exec"
	"syscall"
)

func openPath(target Target) error {
	// explorer.exe exits with status 1 even when it opened the window, so Start
	// is the success signal; waiting on the process is not.
	if err := explorerCommand(target).Start(); err != nil {
		return fmt.Errorf("open File Explorer: %w", err)
	}
	return nil
}

func explorerCommand(target Target) *exec.Cmd {
	// Explorer requires /select, outside the quotes. Go quotes the whole
	// argument when a path contains spaces, which opens Documents instead.
	// Always quote the validated path because Explorer also separates on commas.
	argument := `"` + target.Path + `"`
	if target.SelectItem {
		argument = "/select," + argument
	}
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "explorer.exe " + argument}
	return cmd
}
