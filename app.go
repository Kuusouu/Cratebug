package main

import "github.com/Kuusouu/Cratebug/internal/discovery"

// App exposes the small backend surface used by the frontend.
type App struct{}

// Creates the application binding.
func NewApp() *App {
	return &App{}
}

// Confirms that the frontend can reach the Go application.
func (a *App) RuntimeStatus() string {
	return "Go backend connected"
}

// Returns the read-only catalog discovered beneath modRoot.
func (a *App) ScanLibrary(modRoot string) (discovery.Library, error) {
	return discovery.Scan(modRoot)
}
