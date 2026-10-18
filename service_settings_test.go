package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/metadata"
)

func TestSetLibraryProviderPersistsAndRejectsUnknownValues(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	if err := app.settings.SetLibraryProvider("steam"); err != nil {
		t.Fatal(err)
	}

	// Act
	state := app.settings.LoadMetadata()
	rejectErr := app.settings.SetLibraryProvider("egs")

	// Assert
	if state.Document.Settings.LibraryProvider != "steam" {
		t.Errorf("Settings.LibraryProvider = %q, want %q", state.Document.Settings.LibraryProvider, "steam")
	}
	if rejectErr == nil {
		t.Error("SetLibraryProvider() succeeded for an unknown provider, want an error")
	}
}

func TestTagLifecycleThroughSettingsService(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := testApp(t, false)
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	entryID := library.Entries[0].ID

	// Act
	tag, err := app.settings.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.settings.AssignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	afterAssign := app.settings.LoadMetadata().Document
	if err := app.settings.UnassignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	afterUnassign := app.settings.LoadMetadata().Document

	// Assert
	modID, ok := afterAssign.FindModByScannerID(entryID)
	if !ok {
		t.Fatal("assigned mod is not tracked in persisted metadata")
	}
	if got := afterAssign.Mods[modID].Tags; len(got) != 1 || got[0] != tag.ID {
		t.Errorf("Tags after assign = %#v, want [%q]", got, tag.ID)
	}
	if got := afterUnassign.Mods[modID].Tags; len(got) != 0 {
		t.Errorf("Tags after unassign = %#v, want none", got)
	}
}

func TestLoadMetadataRecoversFromACorruptedFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	app := testAppWithStore(t, false, metadata.NewStore(path))
	// Set it twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := app.library.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	if err := app.library.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	state := app.settings.LoadMetadata()

	// Assert
	if !state.Recovered {
		t.Error("Recovered = false, want true after loading a corrupted file")
	}
	if state.RecoveryReason == "" {
		t.Error("RecoveryReason is empty, want an explanation of the corruption")
	}
	if state.Document.Settings.ModRoot != `C:\Mods` {
		t.Errorf("recovered Settings.ModRoot = %q, want %q", state.Document.Settings.ModRoot, `C:\Mods`)
	}
}
