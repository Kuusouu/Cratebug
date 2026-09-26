//go:build !windows && !linux

package mutation

// WindowsGameRunningChecker provides a fallback for unsupported platforms.
type WindowsGameRunningChecker struct{}

func (WindowsGameRunningChecker) IsGameRunning() (bool, error) {
	return false, nil
}

// NewGameRunningChecker creates a fallback game-running detector.
func NewGameRunningChecker() GameRunningChecker {
	return WindowsGameRunningChecker{}
}
