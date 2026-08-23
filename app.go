package main

import (
	"fmt"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

// Exposes the small backend surface used by the frontend.
type App struct {
	mutationExecutor mutation.Executor
	metadataStore    metadata.Store
}

// Creates the application binding.
func NewApp() (*App, error) {
	path, err := metadata.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve metadata storage location: %w", err)
	}
	return newApp(mutation.WindowsGameRunningChecker{}, metadata.NewStore(path)), nil
}

// Lets tests inject a deterministic game-running detector and a disposable
// metadata store without exposing either to Wails.
func newApp(gameRunningChecker mutation.GameRunningChecker, metadataStore metadata.Store) *App {
	return &App{
		mutationExecutor: mutation.NewExecutor(gameRunningChecker),
		metadataStore:    metadataStore,
	}
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
	return a.executeAndReconcile(operation)
}

// Changes one current scanner entry's filename-based priority.
func (a *App) SetModPriority(modRoot, entryID string, priority int) (mutation.Result, error) {
	operation := mutation.NewSetPriorityOperation(modRoot, entryID, priority)
	return a.executeAndReconcile(operation)
}

// Moves one current scanner entry to an existing scanner-known folder.
func (a *App) MoveMod(modRoot, entryID, destinationFolder string) (mutation.Result, error) {
	operation := mutation.NewMoveModOperation(modRoot, entryID, destinationFolder)
	return a.executeAndReconcile(operation)
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

// Deletes one current scanner entry through the Windows Recycle Bin.
func (a *App) DeleteMod(modRoot, entryID string, confirmed bool) (mutation.Result, error) {
	operation := mutation.NewDeleteModOperation(modRoot, entryID, confirmed)
	return a.mutationExecutor.Execute(operation)
}

// Executes operation, then re-points any persisted metadata (currently tag
// assignments) from its previous scanner ID to its new one so a rename,
// priority change, or move does not orphan a mod's metadata.
//
// A folder rename or move changes the scanner ID of every mod it contains,
// but its Result reports only the folder's own old and new paths, not a
// per-mod ID pair, so those operations call the executor directly instead of
// this helper. Metadata for mods inside a renamed or moved folder is not
// reconciled; this matches the existing frontend limitation described in
// docs/reviews/phase-4-review.md.
func (a *App) executeAndReconcile(operation mutation.Operation) (mutation.Result, error) {
	result, err := a.mutationExecutor.Execute(operation)
	if err != nil {
		return result, err
	}
	if result.PreviousID == "" || result.PreviousID == result.ID {
		return result, nil
	}

	doc := a.loadMetadataDocument()
	if !doc.ReconcileMod(result.PreviousID, result.ID) {
		return result, nil
	}
	if err := a.metadataStore.Save(doc); err != nil {
		return result, fmt.Errorf("reconcile mod metadata: %w", err)
	}
	return result, nil
}

// MetadataState is the persisted document plus whether it had to be
// recovered from a damaged file, so the frontend can surface that as
// actionable feedback instead of silently losing track of the event.
type MetadataState struct {
	Document       metadata.Document `json:"document"`
	Recovered      bool              `json:"recovered"`
	RecoveryReason string            `json:"recoveryReason,omitempty"`
}

// Returns the persisted settings, tag catalog, and per-mod tag assignments.
func (a *App) LoadMetadata() MetadataState {
	doc, recovery := a.metadataStore.Load()
	state := MetadataState{Document: doc, Recovered: recovery.Recovered}
	if recovery.Cause != nil {
		state.RecoveryReason = recovery.Cause.Error()
	}
	return state
}

// Loads the persisted document for an operation that only needs to read or
// mutate it, discarding recovery details that LoadMetadata reports instead.
func (a *App) loadMetadataDocument() metadata.Document {
	doc, _ := a.metadataStore.Load()
	return doc
}

// Persists the selected mod root so the library does not need to be
// reselected the next time Cratebug launches.
func (a *App) SetModRoot(modRoot string) error {
	doc := a.loadMetadataDocument()
	doc.Settings.ModRoot = modRoot
	return a.metadataStore.Save(doc)
}

// Persists the appearance theme (system, light, or dark).
func (a *App) SetTheme(theme string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetTheme(theme); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Persists the view mode Cratebug opens to on the next launch.
func (a *App) SetDefaultViewMode(mode string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetDefaultViewMode(mode); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Persists the accent color override, or clears it if color is empty.
func (a *App) SetAccentColor(color string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetAccentColor(color); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Adds a new tag to the persisted catalog.
func (a *App) CreateTag(name string) (metadata.Tag, error) {
	doc := a.loadMetadataDocument()
	tag, err := doc.CreateTag(name)
	if err != nil {
		return metadata.Tag{}, err
	}
	if err := a.metadataStore.Save(doc); err != nil {
		return metadata.Tag{}, err
	}
	return tag, nil
}

// Renames an existing tag in the persisted catalog.
func (a *App) RenameTag(id, name string) error {
	doc := a.loadMetadataDocument()
	if err := doc.RenameTag(id, name); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Removes a tag from the catalog and every mod it was assigned to.
func (a *App) DeleteTag(id string) error {
	doc := a.loadMetadataDocument()
	if err := doc.DeleteTag(id); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Assigns an existing catalog tag to one current scanner entry, establishing
// the entry's persistent identity first if this is the first metadata
// recorded for it.
func (a *App) AssignModTag(entryID, tagID string) error {
	doc := a.loadMetadataDocument()
	modID, err := doc.EnsureMod(entryID)
	if err != nil {
		return err
	}
	if err := doc.AssignTag(modID, tagID); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Removes a tag assignment from one current scanner entry.
func (a *App) UnassignModTag(entryID, tagID string) error {
	doc := a.loadMetadataDocument()
	modID, err := doc.EnsureMod(entryID)
	if err != nil {
		return err
	}
	if err := doc.UnassignTag(modID, tagID); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}
