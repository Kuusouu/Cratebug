//go:build windows

package uassettool

import (
	"os/exec"
	"syscall"
)

// WorkerExecutableName is the filename of the UAssetTool binary on Windows.
const (
	WorkerExecutableName       = "UAssetTool.exe"
	WorkerDevelopmentDirectory = "uassettool"
)

func setHideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
