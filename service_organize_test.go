package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

func TestSetModEnabledBlocksRunningGame(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := testApp(t, true)
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = app.organize.SetModEnabled(root, library.Entries[0].ID, false)

	// Assert
	if !errors.Is(err, mutation.ErrGameRunning) {
		t.Fatalf("SetModEnabled() error = %v, want ErrGameRunning", err)
	}
	if _, err := os.Lstat(primaryPath); err != nil {
		t.Errorf("enabled primary is missing: %v", err)
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
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	result, err := app.organize.SetModEnabled(root, library.Entries[0].ID, true)
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
				_, err := app.organize.RenameMod(root, entryID, "Renamed")
				return err
			},
		},
		{
			name: "priority",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.SetModPriority(root, entryID, 2)
				return err
			},
		},
		{
			name: "mod move",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.MoveMod(root, entryID, "destination")
				return err
			},
		},
		{
			name: "folder create",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.CreateFolder(root, "", "created")
				return err
			},
		},
		{
			name: "folder rename",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.RenameFolder(root, "source", "renamed")
				return err
			},
		},
		{
			name: "folder move",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.MoveFolder(root, "source", "destination")
				return err
			},
		},
		{
			name: "deletion",
			run: func(app *App, root, entryID string) error {
				_, err := app.organize.DeleteMod(root, entryID, true)
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
			library, err := app.library.ScanLibrary(root)
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

// Confirms OrganizeService wires mutation.Result.PreviousID/ID into metadata
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
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	entryID := library.Entries[0].ID

	tag, err := app.settings.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.settings.AssignModTag(entryID, tag.ID); err != nil {
		t.Fatal(err)
	}
	before := app.settings.LoadMetadata().Document
	modID, ok := before.FindModByScannerID(entryID)
	if !ok {
		t.Fatal("assigned mod is not tracked in persisted metadata")
	}

	// Act
	result, err := app.organize.RenameMod(root, entryID, "Renamed")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	after := app.settings.LoadMetadata().Document
	if got := after.Mods[modID].Tags; len(got) != 1 || got[0] != tag.ID {
		t.Errorf("Tags after rename = %#v, want [%q]", got, tag.ID)
	}
	if after.Mods[modID].ScannerID != result.ID {
		t.Errorf("ScannerID after rename = %q, want %q", after.Mods[modID].ScannerID, result.ID)
	}
}
