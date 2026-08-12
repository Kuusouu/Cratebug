package main

import (
	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

// Exposes the small backend surface used by the frontend.
type App struct {
	mutationExecutor mutation.Executor
}

// Creates the application binding.
func NewApp() *App {
	return newApp(mutation.WindowsGameRunningChecker{})
}

// Lets tests inject a deterministic game-running detector without exposing it to Wails.
func newApp(gameRunningChecker mutation.GameRunningChecker) *App {
	return &App{mutationExecutor: mutation.NewExecutor(gameRunningChecker)}
}

// Confirms that the frontend can reach the Go application.
func (a *App) RuntimeStatus() string {
	return "Go backend connected"
}

// Returns the read-only catalog discovered beneath modRoot.
func (a *App) ScanLibrary(modRoot string) (discovery.Library, error) {
	return discovery.Scan(modRoot)
}

// Changes one current scanner entry to the requested enabled state.
// The primary path is a scanner-relative identifier, never an arbitrary filesystem path.
func (a *App) SetModEnabled(modRoot, primaryPath string, enabled bool) (mutation.Result, error) {
	operation := mutation.NewSetEnabledOperation(modRoot, primaryPath, enabled)
	return a.mutationExecutor.Execute(operation)
}
