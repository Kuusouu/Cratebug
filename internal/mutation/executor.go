package mutation

import (
	"errors"
	"fmt"
)

// Describes whether an operation may run while Marvel Rivals is open.
type GameRunningRequirement uint8

const (
	AllowedWhileGameRunning GameRunningRequirement = iota
	BlockedWhileGameRunning
)

// Returned before a blocked operation changes the filesystem.
var ErrGameRunning = errors.New("Marvel Rivals is running. Close it before changing mods.")

// Reports whether the established Marvel Rivals executable is running.
type GameRunningChecker interface {
	IsGameRunning() (bool, error)
}

// Represents a mutation that declares its game-running safety requirement.
type Operation interface {
	GameRunningRequirement() GameRunningRequirement
	Execute() (Result, error)
}

// Applies cross-cutting mutation policy before an operation can touch the filesystem.
type Executor struct {
	gameRunningChecker GameRunningChecker
}

// Changes one current scanner entry to the requested enabled state.
type SetEnabledOperation struct {
	modRoot string
	entryID string
	enabled bool
}

// Renames one scanner-discovered mod while keeping its recognized bundle synchronized.
type RenameModOperation struct {
	modRoot string
	entryID string
	name    string
}

// Changes one scanner-discovered mod's filename-based priority.
type SetPriorityOperation struct {
	modRoot  string
	entryID  string
	priority int
}

// Moves one scanner-discovered mod to a scanner-known physical folder.
type MoveModOperation struct {
	modRoot           string
	entryID           string
	destinationFolder string
}

// Creates one physical folder beneath a scanner-known parent folder.
type CreateFolderOperation struct {
	modRoot      string
	parentFolder string
	name         string
}

// Renames one scanner-known physical folder.
type RenameFolderOperation struct {
	modRoot string
	folder  string
	name    string
}

// Moves one scanner-known physical folder beneath a scanner-known parent.
type MoveFolderOperation struct {
	modRoot           string
	folder            string
	destinationParent string
}

// Deletes one scanner-discovered bundle through the Windows Recycle Bin.
type DeleteModOperation struct {
	modRoot   string
	entryID   string
	confirmed bool
}

// Creates a mutation executor with the supplied game-running detector.
func NewExecutor(gameRunningChecker GameRunningChecker) Executor {
	return Executor{gameRunningChecker: gameRunningChecker}
}

// Creates the narrow operation exposed by the Phase 3 application boundary.
func NewSetEnabledOperation(modRoot, entryID string, enabled bool) SetEnabledOperation {
	return SetEnabledOperation{modRoot: modRoot, entryID: entryID, enabled: enabled}
}

// Creates the narrow rename operation exposed by the Phase 4 application boundary.
func NewRenameModOperation(modRoot, entryID, name string) RenameModOperation {
	return RenameModOperation{modRoot: modRoot, entryID: entryID, name: name}
}

// Creates the narrow priority operation exposed by the Phase 4 application boundary.
func NewSetPriorityOperation(modRoot, entryID string, priority int) SetPriorityOperation {
	return SetPriorityOperation{modRoot: modRoot, entryID: entryID, priority: priority}
}

// Creates the narrow mod-move operation exposed by the Phase 4 application boundary.
func NewMoveModOperation(modRoot, entryID, destinationFolder string) MoveModOperation {
	return MoveModOperation{modRoot: modRoot, entryID: entryID, destinationFolder: destinationFolder}
}

// Creates the narrow folder-creation operation exposed by the Phase 4 application boundary.
func NewCreateFolderOperation(modRoot, parentFolder, name string) CreateFolderOperation {
	return CreateFolderOperation{modRoot: modRoot, parentFolder: parentFolder, name: name}
}

// Creates the narrow folder-rename operation exposed by the Phase 4 application boundary.
func NewRenameFolderOperation(modRoot, folder, name string) RenameFolderOperation {
	return RenameFolderOperation{modRoot: modRoot, folder: folder, name: name}
}

// Creates the narrow folder-move operation exposed by the Phase 4 application boundary.
func NewMoveFolderOperation(modRoot, folder, destinationParent string) MoveFolderOperation {
	return MoveFolderOperation{modRoot: modRoot, folder: folder, destinationParent: destinationParent}
}

// Creates the narrow Recycle Bin deletion operation exposed by the Phase 4 application boundary.
func NewDeleteModOperation(modRoot, entryID string, confirmed bool) DeleteModOperation {
	return DeleteModOperation{modRoot: modRoot, entryID: entryID, confirmed: confirmed}
}

// Applies the operation only after its game-running restriction has been enforced.
func (executor Executor) Execute(operation Operation) (Result, error) {
	switch operation.GameRunningRequirement() {
	case AllowedWhileGameRunning:
		return operation.Execute()
	case BlockedWhileGameRunning:
		if executor.gameRunningChecker == nil {
			return Result{}, errors.New("game-running detector is unavailable; mutation blocked")
		}

		// Fail closed because an unavailable detector cannot safely authorize a filesystem mutation.
		gameRunning, err := executor.gameRunningChecker.IsGameRunning()
		if err != nil {
			return Result{}, fmt.Errorf("check whether Marvel Rivals is running: %w", err)
		}

		if gameRunning {
			return Result{}, ErrGameRunning
		}
	default:
		return Result{}, fmt.Errorf("unknown game-running requirement: %d", operation.GameRunningRequirement())
	}

	return operation.Execute()
}

// Blocks mod filename changes while the game may have their archives open.
func (operation SetEnabledOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Performs the existing primary-only transition after the executor has authorized it.
func (operation SetEnabledOperation) Execute() (Result, error) {
	return setEnabled(operation.modRoot, operation.entryID, operation.enabled)
}

// Blocks bundle filename changes while the game may have their archives open.
func (operation RenameModOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Performs the planned bundle rename after the executor has authorized it.
func (operation RenameModOperation) Execute() (Result, error) {
	return renameMod(operation.modRoot, operation.entryID, operation.name)
}

// Blocks priority filename changes while the game may have their archives open.
func (operation SetPriorityOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Performs the planned priority change after the executor has authorized it.
func (operation SetPriorityOperation) Execute() (Result, error) {
	return setPriority(operation.modRoot, operation.entryID, operation.priority)
}

// Blocks a bundle move while the game may have its archive files open.
func (operation MoveModOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Moves the scanner-selected bundle after game-running protection succeeds.
func (operation MoveModOperation) Execute() (Result, error) {
	return moveMod(operation.modRoot, operation.entryID, operation.destinationFolder)
}

// Blocks folder creation while the game may be using the mod directory.
func (operation CreateFolderOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Creates the requested child folder after game-running protection succeeds.
func (operation CreateFolderOperation) Execute() (Result, error) {
	return createFolder(operation.modRoot, operation.parentFolder, operation.name)
}

// Blocks folder renames while the game may be using the mod directory.
func (operation RenameFolderOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Renames the scanner-selected folder after game-running protection succeeds.
func (operation RenameFolderOperation) Execute() (Result, error) {
	return renameFolder(operation.modRoot, operation.folder, operation.name)
}

// Blocks folder moves while the game may be using the mod directory.
func (operation MoveFolderOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Moves the scanner-selected folder after game-running protection succeeds.
func (operation MoveFolderOperation) Execute() (Result, error) {
	return moveFolderToParent(operation.modRoot, operation.folder, operation.destinationParent)
}

// Blocks deletion while the game may have the bundle's archive files open.
func (operation DeleteModOperation) GameRunningRequirement() GameRunningRequirement {
	return BlockedWhileGameRunning
}

// Sends the scanner-selected bundle to the Recycle Bin after safety checks succeed.
func (operation DeleteModOperation) Execute() (Result, error) {
	return deleteMod(operation.modRoot, operation.entryID, operation.confirmed)
}
