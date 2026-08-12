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
