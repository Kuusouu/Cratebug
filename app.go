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
// The entry ID is scanner-issued, never an arbitrary filesystem path.
func (a *App) SetModEnabled(modRoot, entryID string, enabled bool) (mutation.Result, error) {
	operation := mutation.NewSetEnabledOperation(modRoot, entryID, enabled)
	return a.mutationExecutor.Execute(operation)
}

// Renames one current scanner entry without exposing arbitrary filesystem paths.
func (a *App) RenameMod(modRoot, entryID, name string) (mutation.Result, error) {
	operation := mutation.NewRenameModOperation(modRoot, entryID, name)
	return a.mutationExecutor.Execute(operation)
}

// Changes one current scanner entry's filename-based priority.
func (a *App) SetModPriority(modRoot, entryID string, priority int) (mutation.Result, error) {
	operation := mutation.NewSetPriorityOperation(modRoot, entryID, priority)
	return a.mutationExecutor.Execute(operation)
}

// Moves one current scanner entry to an existing scanner-known folder.
func (a *App) MoveMod(modRoot, entryID, destinationFolder string) (mutation.Result, error) {
	operation := mutation.NewMoveModOperation(modRoot, entryID, destinationFolder)
	return a.mutationExecutor.Execute(operation)
}

// Creates one folder beneath the root or an existing scanner-known folder.
func (a *App) CreateFolder(modRoot, parentFolder, name string) (mutation.Result, error) {
	operation := mutation.NewCreateFolderOperation(modRoot, parentFolder, name)
	return a.mutationExecutor.Execute(operation)
}

// Renames one scanner-known physical folder.
func (a *App) RenameFolder(modRoot, folder, name string) (mutation.Result, error) {
	operation := mutation.NewRenameFolderOperation(modRoot, folder, name)
	return a.mutationExecutor.Execute(operation)
}

// Moves one scanner-known physical folder beneath the root or another scanner-known folder.
func (a *App) MoveFolder(modRoot, folder, destinationParent string) (mutation.Result, error) {
	operation := mutation.NewMoveFolderOperation(modRoot, folder, destinationParent)
	return a.mutationExecutor.Execute(operation)
}
