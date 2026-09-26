//go:build !windows

package uassettool

import "os/exec"

// WorkerExecutableName is the filename of the UAssetTool binary on non-Windows platforms.
const (
	WorkerExecutableName       = "UAssetTool"
	WorkerDevelopmentDirectory = "uassettool-linux"
)

func setHideWindow(cmd *exec.Cmd) {}
