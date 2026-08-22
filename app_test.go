package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/metadata"
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

func TestRuntimeStatus(t *testing.T) {
	// Arrange
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))

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

	app := newApp(staticGameRunningChecker{gameRunning: true}, testMetadataStore(t))
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

	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))

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

	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))
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
			app := newApp(staticGameRunningChecker{gameRunning: true}, testMetadataStore(t))
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
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))

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
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))
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
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t))
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
	app := newApp(staticGameRunningChecker{}, metadata.NewStore(path))
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
