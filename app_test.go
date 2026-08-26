package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type staticGameRunningChecker struct {
	gameRunning bool
}

func (checker staticGameRunningChecker) IsGameRunning() (bool, error) {
	return checker.gameRunning, nil
}

func testMetadataStore(t *testing.T) metadata.Store {
	t.Helper()
	return metadata.NewStore(filepath.Join(t.TempDir(), "metadata.json"))
}

func testApp(t *testing.T, gameRunning bool) *App {
	t.Helper()
	emptyTable := modtype.CharacterTable{}
	return newApp(staticGameRunningChecker{gameRunning: gameRunning}, testMetadataStore(t), nil, &emptyTable, nil)
}

func testAppWithStore(t *testing.T, gameRunning bool, store metadata.Store) *App {
	t.Helper()
	emptyTable := modtype.CharacterTable{}
	return newApp(staticGameRunningChecker{gameRunning: gameRunning}, store, nil, &emptyTable, nil)
}

func TestRuntimeStatus(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	got := app.RuntimeStatus()

	// Assert
	const want = "Go backend connected"
	if got != want {
		t.Fatalf("RuntimeStatus() = %q, want %q", got, want)
	}
}

func TestSetModEnabledBlocksRunningGame(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := testApp(t, true)
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = app.SetModEnabled(root, library.Entries[0].ID, false)

	// Assert
	if !errors.Is(err, mutation.ErrGameRunning) {
		t.Fatalf("SetModEnabled() error = %v, want ErrGameRunning", err)
	}
	if _, err := os.Lstat(primaryPath); err != nil {
		t.Errorf("enabled primary is missing: %v", err)
	}
}

func TestScanLibrary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := testApp(t, false)

	// Act
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if library.Root != root {
		t.Errorf("Root = %q, want %q", library.Root, root)
	}
	if len(library.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(library.Entries))
	}
	entry := library.Entries[0]
	if entry.DisplayName != "Example" {
		t.Errorf("DisplayName = %q, want %q", entry.DisplayName, "Example")
	}
	if entry.State != discovery.StateEnabled {
		t.Errorf("State = %q, want %q", entry.State, discovery.StateEnabled)
	}
}

func TestSetModEnabled(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.bak_bento")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := testApp(t, false)
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	result, err := app.SetModEnabled(root, library.Entries[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if result.PrimaryPath != "Example_9999999_P.pak" || result.State != discovery.StateEnabled {
		t.Errorf("result = %#v, want enabled Example_9999999_P.pak", result)
	}
}

func TestOrganizationOperationsBlockRunningGame(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(app *App, root, entryID string) error
	}{
		{
			name: "rename",
			run: func(app *App, root, entryID string) error {
				_, err := app.RenameMod(root, entryID, "Renamed")
				return err
			},
		},
		{
			name: "priority",
			run: func(app *App, root, entryID string) error {
				_, err := app.SetModPriority(root, entryID, 2)
				return err
			},
		},
		{
			name: "mod move",
			run: func(app *App, root, entryID string) error {
				_, err := app.MoveMod(root, entryID, "destination")
				return err
			},
		},
		{
			name: "folder create",
			run: func(app *App, root, entryID string) error {
				_, err := app.CreateFolder(root, "", "created")
				return err
			},
		},
		{
			name: "folder rename",
			run: func(app *App, root, entryID string) error {
				_, err := app.RenameFolder(root, "source", "renamed")
				return err
			},
		},
		{
			name: "folder move",
			run: func(app *App, root, entryID string) error {
				_, err := app.MoveFolder(root, "source", "destination")
				return err
			},
		},
		{
			name: "deletion",
			run: func(app *App, root, entryID string) error {
				_, err := app.DeleteMod(root, entryID, true)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			primaryPath := filepath.Join(root, "Example_9999999_P.pak")
			if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "source"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "destination"), 0o700); err != nil {
				t.Fatal(err)
			}
			app := testApp(t, true)
			library, err := app.ScanLibrary(root)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			err = test.run(app, root, library.Entries[0].ID)

			// Assert
			if !errors.Is(err, mutation.ErrGameRunning) {
				t.Fatalf("organization operation error = %v, want ErrGameRunning", err)
			}
			if _, err := os.Lstat(primaryPath); err != nil {
				t.Errorf("blocked operation changed primary: %v", err)
			}
		})
	}
}

func TestSetModRootPersistsAcrossLoads(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	if err := app.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	state := app.LoadMetadata()

	// Assert
	if state.Document.Settings.ModRoot != `C:\Mods` {
		t.Errorf("Settings.ModRoot = %q, want %q", state.Document.Settings.ModRoot, `C:\Mods`)
	}
}

func TestTagLifecycleThroughApp(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := testApp(t, false)
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	entryID := library.Entries[0].ID

	// Act
	tag, err := app.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.AssignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	afterAssign := app.LoadMetadata().Document
	if err := app.UnassignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	afterUnassign := app.LoadMetadata().Document

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

// Confirms App wires mutation.Result.PreviousID/ID into metadata
// reconciliation, so a tag assigned before a rename is still assigned to the
// same mod's persistent identity afterward.
func TestRenameModReconcilesPersistedTagAssignment(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := testApp(t, false)
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	entryID := library.Entries[0].ID

	tag, err := app.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.AssignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	before := app.LoadMetadata().Document
	modID, ok := before.FindModByScannerID(entryID)
	if !ok {
		t.Fatal("assigned mod is not tracked in persisted metadata")
	}

	// Act
	result, err := app.RenameMod(root, entryID, "Renamed")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	after := app.LoadMetadata().Document
	if got := after.Mods[modID].Tags; len(got) != 1 || got[0] != tag.ID {
		t.Errorf("Tags after rename = %#v, want [%q]", got, tag.ID)
	}
	if after.Mods[modID].ScannerID != result.ID {
		t.Errorf("ScannerID after rename = %q, want %q", after.Mods[modID].ScannerID, result.ID)
	}
}

func TestLoadMetadataRecoversFromACorruptedFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	app := testAppWithStore(t, false, metadata.NewStore(path))
	// Set it twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := app.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	if err := app.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	state := app.LoadMetadata()

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

func TestAppShutdown(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act & Assert (shutdown must complete cleanly without panic)
	app.shutdown(context.Background())
}

func TestClassifyLibrary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	table := modtype.CharacterTable{
		CharacterNames: map[string]string{"1044": "Blade"},
	}
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t), nil, &table, nil)
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	results, err := app.ClassifyLibrary(root, library.Entries)

	// Assert
	if err != nil {
		t.Fatalf("ClassifyLibrary() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	// Without a live worker running in this unit test, it degrades to CategoryUnknown gracefully
	if results[library.Entries[0].ID].Category != modtype.CategoryUnknown {
		t.Errorf("Category = %q, want CategoryUnknown", results[library.Entries[0].ID].Category)
	}
}

func TestAppInstallLifecycle(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	sourcePak := filepath.Join(sourceDir, "Punisher_P.pak")
	if err := os.WriteFile(sourcePak, []byte("punisher"), 0o600); err != nil {
		t.Fatalf("write source pak: %v", err)
	}

	app := testApp(t, false)

	// Act - Prepare
	preview, err := app.PrepareInstall(modRoot, []string{sourcePak}, "Characters/Punisher")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}

	// Assert - Prepare
	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item in preview, got %d", len(preview.Items))
	}
	if preview.Items[0].ModName != "Punisher" {
		t.Errorf("expected ModName Punisher, got %q", preview.Items[0].ModName)
	}

	// Act - Apply
	applyItems := []install.ApplyItem{
		{
			ID:                preview.Items[0].ID,
			ModName:           "Punisher",
			DestinationFolder: "Characters/Punisher",
			Overwrite:         false,
		},
	}
	result, err := app.ApplyInstall(modRoot, preview.SessionID, applyItems)

	// Assert - Apply
	if err != nil {
		t.Fatalf("ApplyInstall failed: %v", err)
	}
	if len(result.InstalledEntryIDs) != 1 {
		t.Errorf("expected 1 installed entry ID, got %d", len(result.InstalledEntryIDs))
	}

	destPak := filepath.Join(modRoot, "Characters", "Punisher", "Punisher_P.pak")
	if _, err := os.Stat(destPak); err != nil {
		t.Fatalf("expected installed file missing: %s", destPak)
	}
}

func TestAppInstallCancel(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	sourcePak := filepath.Join(sourceDir, "Groot_P.pak")
	if err := os.WriteFile(sourcePak, []byte("groot"), 0o600); err != nil {
		t.Fatalf("write source pak: %v", err)
	}

	app := testApp(t, false)

	preview, err := app.PrepareInstall(modRoot, []string{sourcePak}, "")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}

	// Act - Cancel
	err = app.CancelInstall(preview.SessionID)

	// Assert - Cancel
	if err != nil {
		t.Fatalf("CancelInstall failed: %v", err)
	}

	// Attempting to apply after cancel should fail because session is removed
	applyItems := []install.ApplyItem{
		{
			ID:                preview.Items[0].ID,
			ModName:           "Groot",
			DestinationFolder: "",
			Overwrite:         false,
		},
	}
	_, err = app.ApplyInstall(modRoot, preview.SessionID, applyItems)
	if err == nil {
		t.Fatalf("expected ApplyInstall to fail after CancelInstall, but it succeeded")
	}
}
