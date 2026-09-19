package main

import "github.com/Kuusouu/Cratebug/internal/reveal"

// Opens one root-relative folder, or the library root, in File Explorer.
func (a *App) OpenFolderInExplorer(modRoot, folder string) error {
	return reveal.OpenFolder(modRoot, folder)
}

// Opens File Explorer with the scanned mod selected.
func (a *App) OpenModInExplorer(modRoot, entryID string) error {
	return reveal.OpenMod(modRoot, entryID)
}
