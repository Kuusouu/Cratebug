package main

import (
	"fmt"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

// Renames, moves, deletes, and toggles mods and folders through the gated
// mutation executor.
type OrganizeService struct {
	*core

	mutationExecutor mutation.Executor
}

func newOrganizeService(c *core) *OrganizeService {
	return &OrganizeService{core: c, mutationExecutor: mutation.NewExecutor(c.game)}
}

// Changes one current scanner entry to the requested enabled state.
// The entry ID is scanner-issued, never an arbitrary filesystem path.
func (s *OrganizeService) SetModEnabled(modRoot, entryID string, enabled bool) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewSetEnabledOperation(modRoot, entryID, enabled)
		return s.mutationExecutor.Execute(operation)
	})
}

// Renames one current scanner entry without exposing arbitrary filesystem paths.
func (s *OrganizeService) RenameMod(modRoot, entryID, name string) (mutation.Result, error) {
	operation := mutation.NewRenameModOperation(modRoot, entryID, name)
	return s.executeAndReconcile(operation)
}

// Changes one current scanner entry's filename-based priority.
func (s *OrganizeService) SetModPriority(modRoot, entryID string, priority int) (mutation.Result, error) {
	operation := mutation.NewSetPriorityOperation(modRoot, entryID, priority)
	return s.executeAndReconcile(operation)
}

// Moves one current scanner entry to an existing scanner-known folder.
func (s *OrganizeService) MoveMod(modRoot, entryID, destinationFolder string) (mutation.Result, error) {
	operation := mutation.NewMoveModOperation(modRoot, entryID, destinationFolder)
	return s.executeAndReconcile(operation)
}

// Creates one folder beneath the root or an existing scanner-known folder.
func (s *OrganizeService) CreateFolder(modRoot, parentFolder, name string) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewCreateFolderOperation(modRoot, parentFolder, name)
		return s.mutationExecutor.Execute(operation)
	})
}

// Renames one scanner-known physical folder.
func (s *OrganizeService) RenameFolder(modRoot, folder, name string) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewRenameFolderOperation(modRoot, folder, name)
		return s.mutationExecutor.Execute(operation)
	})
}

// Moves one scanner-known physical folder beneath the root or another scanner-known folder.
func (s *OrganizeService) MoveFolder(modRoot, folder, destinationParent string) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewMoveFolderOperation(modRoot, folder, destinationParent)
		return s.mutationExecutor.Execute(operation)
	})
}

// Deletes one current scanner entry through the Windows Recycle Bin.
func (s *OrganizeService) DeleteMod(modRoot, entryID string, confirmed bool) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewDeleteModOperation(modRoot, entryID, confirmed)
		return s.mutationExecutor.Execute(operation)
	})
}

// Deletes one scanner-known physical folder and its contents through the Windows Recycle Bin.
func (s *OrganizeService) DeleteFolder(modRoot, folder string, confirmed bool) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		operation := mutation.NewDeleteFolderOperation(modRoot, folder, confirmed)
		return s.mutationExecutor.Execute(operation)
	})
}

// Reports whether one scanner-known physical folder holds no entries at all.
func (s *OrganizeService) IsFolderEmpty(modRoot, folder string) (bool, error) {
	return mutation.FolderIsEmpty(modRoot, folder)
}

// Executes operation, then re-points any persisted metadata (currently tag
// assignments) from its previous scanner ID to its new one so a rename,
// priority change, or move does not orphan a mod's metadata.
//
// A folder rename or move changes the scanner ID of every mod it contains,
// but its Result reports only the folder's own old and new paths, not a
// per-mod ID pair, so those operations call the executor directly instead of
// this helper. These operations do not reconcile metadata for mods in the folder.
func (s *OrganizeService) executeAndReconcile(operation mutation.Operation) (mutation.Result, error) {
	return suppressWatcherResult(s.core, func() (mutation.Result, error) {
		result, err := s.mutationExecutor.Execute(operation)
		if err != nil {
			return result, err
		}
		if result.PreviousID == "" || result.PreviousID == result.ID {
			return result, nil
		}

		err = s.meta.update(func(doc *metadata.Document) error {
			if !doc.ReconcileMod(result.PreviousID, result.ID) {
				return errMetadataUnchanged
			}
			return nil
		})
		if err != nil {
			return result, fmt.Errorf("reconcile mod metadata: %w", err)
		}
		return result, nil
	})
}
