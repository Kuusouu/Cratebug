package mutation

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// Verifies a rename treats the discovered primary and IoStore sidecars as one bundle.
func TestRenameModRenamesIoStoreBundleAndPreservesDisabledSuffix(t *testing.T) {
	// Arrange
	root := t.TempDir()
	paths := map[string]string{
		"nested/!Old_9999999_P.pak_crateoff": "primary",
		"nested/!Old_9999999_P.utoc":         "utoc",
		"nested/!Old_9999999_P.ucas":         "ucas",
	}
	for path, contents := range paths {
		writeFile(t, filepath.Join(root, path), contents)
	}
	entryID := scannedEntryID(t, root, "nested/!Old_9999999_P.pak_crateoff")

	// Act
	result, err := renameMod(root, entryID, "Renamed")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	want := map[string]string{
		"nested/!Renamed.pak_crateoff": "primary",
		"nested/!Renamed.utoc":         "utoc",
		"nested/!Renamed.ucas":         "ucas",
	}
	if got := snapshotFiles(t, root, ""); !reflect.DeepEqual(got, want) {
		t.Errorf("renamed bundle = %#v, want %#v", got, want)
	}
	if result.PreviousID != entryID {
		t.Errorf("PreviousID = %q, want %q", result.PreviousID, entryID)
	}
	if result.ID == entryID {
		t.Error("ID did not change after a renamed primary changed scanner identity")
	}
	if result.PrimaryPath != "nested/!Renamed.pak_crateoff" {
		t.Errorf("PrimaryPath = %q, want renamed disabled primary", result.PrimaryPath)
	}
}

// Covers both established priority encodings without relying on a UI representation.
func TestSetPriorityUsesCompatibleFilenameForms(t *testing.T) {
	for _, test := range []struct {
		name        string
		source      string
		priority    int
		destination string
	}{
		{name: "zero uses leading bang", source: "Example.pak", priority: 0, destination: "!Example.pak"},
		{name: "positive uses seven nines for one", source: "!Example.pak_disabled", priority: 1, destination: "Example_9999999_P.pak_disabled"},
		{name: "positive adds nines above compatibility minimum", source: "Example_9999999_P.pak", priority: 3, destination: "Example_999999999_P.pak"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			writeFile(t, filepath.Join(root, test.source), "primary")
			entryID := scannedEntryID(t, root, test.source)

			// Act
			result, err := setPriority(root, entryID, test.priority)
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			assertFileContents(t, filepath.Join(root, test.destination), "primary")
			if result.PrimaryPath != test.destination {
				t.Errorf("PrimaryPath = %q, want %q", result.PrimaryPath, test.destination)
			}
			if result.State != discovery.StateDisabled && strings.HasSuffix(test.source, ".pak_disabled") {
				t.Errorf("State = %q, want disabled legacy state", result.State)
			}
		})
	}
}

// Ensures rejected names, priority no-ops, and planned destination collisions cannot alter a bundle.
func TestOrganizationOperationsRejectUnsafeOrCollidingPlansWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(root, entryID string) error
	}{
		{
			name: "path traversal name",
			run: func(root, entryID string) error {
				_, err := renameMod(root, entryID, "../Outside")
				return err
			},
		},
		{
			name: "Windows reserved character",
			run: func(root, entryID string) error {
				_, err := renameMod(root, entryID, "Invalid:Name")
				return err
			},
		},
		{
			name: "Windows reserved device",
			run: func(root, entryID string) error {
				_, err := renameMod(root, entryID, "NUL")
				return err
			},
		},
		{
			name: "Windows reserved device with extension",
			run: func(root, entryID string) error {
				_, err := renameMod(root, entryID, "lpt9.backup")
				return err
			},
		},
		{
			name: "priority no-op",
			run: func(root, entryID string) error {
				_, err := setPriority(root, entryID, 1)
				return err
			},
		},
		{
			name: "priority too large for a filename",
			run: func(root, entryID string) error {
				_, err := setPriority(root, entryID, 1<<60)
				return err
			},
		},
		{
			name: "destination primary collision",
			run: func(root, entryID string) error {
				_, err := renameMod(root, entryID, "Renamed")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "Example_9999999_P.pak"), "primary")
			writeFile(t, filepath.Join(root, "Example_9999999_P.utoc"), "utoc")
			writeFile(t, filepath.Join(root, "Example_9999999_P.ucas"), "ucas")
			if test.name == "destination primary collision" {
				writeFile(t, filepath.Join(root, "Renamed_9999999_P.pak"), "collision")
			}
			before := snapshotFiles(t, root, "")
			entryID := scannedEntryID(t, root, "Example_9999999_P.pak")

			// Act
			err := test.run(root, entryID)

			// Assert
			if err == nil {
				t.Fatal("organization operation succeeded, want error")
			}
			if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
				t.Errorf("rejected operation changed files\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

// Rejects incomplete IoStore bundles because their validity rules are not yet
// established for controlled mutation.
func TestOrganizationOperationsRejectIncompleteIoStoreBundlesWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name  string
		files []string
	}{
		{name: "missing utoc", files: []string{"Example_9999999_P.pak", "Example_9999999_P.ucas"}},
		{name: "missing ucas", files: []string{"Example_9999999_P.pak", "Example_9999999_P.utoc"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, filepath.Join(root, file), file)
			}
			before := snapshotFiles(t, root, "")
			entryID := scannedEntryID(t, root, "Example_9999999_P.pak")

			// Act
			_, err := renameMod(root, entryID, "Renamed")

			// Assert
			if err == nil {
				t.Fatal("renameMod succeeded, want incomplete bundle error")
			}
			if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
				t.Errorf("incomplete bundle changed files\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

// Exercises a mid-bundle failure and verifies rollback restores the original files.
func TestRenameModRollsBackCompletedSidecarMovesAfterFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for path, contents := range map[string]string{
		"Example_9999999_P.pak":  "primary",
		"Example_9999999_P.utoc": "utoc",
		"Example_9999999_P.ucas": "ucas",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	before := snapshotFiles(t, root, "")
	entryID := scannedEntryID(t, root, "Example_9999999_P.pak")
	move := func(source, destination string) error {
		if strings.HasSuffix(destination, ".utoc") {
			return errors.New("injected utoc rename failure")
		}
		return moveFileWithoutReplace(source, destination)
	}

	// Act
	_, err := renameModWithMove(root, entryID, "Renamed", move)

	// Assert
	if err == nil {
		t.Fatal("renameModWithMove succeeded, want injected failure")
	}
	if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
		t.Errorf("rollback did not restore original bundle\nbefore: %#v\nafter: %#v", before, after)
	}
}

// Confirms that a failed rollback reports the actual on-disk partial bundle.
func TestRenameModReconcilesFailedRollback(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for path, contents := range map[string]string{
		"Example_9999999_P.pak":  "primary",
		"Example_9999999_P.utoc": "utoc",
		"Example_9999999_P.ucas": "ucas",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	entryID := scannedEntryID(t, root, "Example_9999999_P.pak")
	move := func(source, destination string) error {
		if strings.HasSuffix(destination, ".utoc") {
			return errors.New("injected utoc rename failure")
		}
		if strings.HasSuffix(source, "Renamed_9999999_P.pak") {
			return errors.New("injected primary rollback failure")
		}
		return moveFileWithoutReplace(source, destination)
	}

	// Act
	result, err := renameModWithMove(root, entryID, "Renamed", move)

	// Assert
	if err == nil {
		t.Fatal("renameModWithMove succeeded, want rollback failure")
	}
	if !strings.Contains(err.Error(), `"Renamed_9999999_P.pak" exists`) {
		t.Errorf("error = %q, want reconciled destination state", err)
	}
	if result.PrimaryPath != "Renamed_9999999_P.pak" {
		t.Errorf("PrimaryPath = %q, want reconciled primary path", result.PrimaryPath)
	}
	assertFileContents(t, filepath.Join(root, "Renamed_9999999_P.pak"), "primary")
	assertFileContents(t, filepath.Join(root, "Example_9999999_P.utoc"), "utoc")
	assertFileContents(t, filepath.Join(root, "Example_9999999_P.ucas"), "ucas")
}
