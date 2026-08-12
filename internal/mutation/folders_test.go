package mutation

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// Verifies a mod move keeps every recognized bundle member in the destination
// folder and returns scanner identities for targeted catalog reconciliation.
func TestMoveModMovesCompleteIoStoreBundle(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for path, contents := range map[string]string{
		"source/Example_9999999_P.pak":  "primary",
		"source/Example_9999999_P.utoc": "utoc",
		"source/Example_9999999_P.ucas": "ucas",
		"destination/.keep":             "folder marker",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	entryID := scannedEntryID(t, root, "source/Example_9999999_P.pak")

	// Act
	result, err := moveMod(root, entryID, "destination")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	want := map[string]string{
		"destination/.keep":                  "folder marker",
		"destination/Example_9999999_P.pak":  "primary",
		"destination/Example_9999999_P.utoc": "utoc",
		"destination/Example_9999999_P.ucas": "ucas",
	}
	if got := snapshotFiles(t, root, ""); !reflect.DeepEqual(got, want) {
		t.Errorf("moved bundle = %#v, want %#v", got, want)
	}
	if result.PreviousID != entryID {
		t.Errorf("PreviousID = %q, want %q", result.PreviousID, entryID)
	}
	if result.PrimaryPath != "destination/Example_9999999_P.pak" {
		t.Errorf("PrimaryPath = %q, want destination primary", result.PrimaryPath)
	}
}

// Covers nested create, rename, and move operations without requiring a UI.
func TestFolderOperationsOrganizeNestedFolders(t *testing.T) {
	// Arrange
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "archive"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Act
	created, err := createFolder(root, "", "source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createFolder(root, created.FolderPath, "child"); err != nil {
		t.Fatal(err)
	}
	renamed, err := renameFolder(root, "source", "active")
	if err != nil {
		t.Fatal(err)
	}
	moved, err := moveFolderToParent(root, renamed.FolderPath, "archive")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if moved.PreviousFolderPath != "active" || moved.FolderPath != "archive/active" {
		t.Errorf("move result = %#v, want active moved under archive", moved)
	}
	if _, err := os.Stat(filepath.Join(root, "archive", "active", "child")); err != nil {
		t.Errorf("nested child did not move with folder: %v", err)
	}
}

// Ensures invalid folder relationships and collisions are rejected before
// changing any primary, sidecar, or directory.
func TestOrganizationMovesRejectUnsafeFoldersWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(root, entryID string) error
	}{
		{
			name: "mod move to unknown folder",
			run: func(root, entryID string) error {
				_, err := moveMod(root, entryID, "missing")
				return err
			},
		},
		{
			name: "mod move traversal folder",
			run: func(root, entryID string) error {
				_, err := moveMod(root, entryID, "../outside")
				return err
			},
		},
		{
			name: "mod move absolute folder",
			run: func(root, entryID string) error {
				_, err := moveMod(root, entryID, filepath.Join(root, "destination"))
				return err
			},
		},
		{
			name: "rename root folder",
			run: func(root, entryID string) error {
				_, err := renameFolder(root, "", "renamed")
				return err
			},
		},
		{
			name: "move folder into descendant",
			run: func(root, entryID string) error {
				_, err := moveFolderToParent(root, "source", "source/child")
				return err
			},
		},
		{
			name: "folder destination collision",
			run: func(root, entryID string) error {
				_, err := renameFolder(root, "source", "destination")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			for path, contents := range map[string]string{
				"source/Example_9999999_P.pak": "primary",
				"source/child/.keep":           "child",
				"destination/.keep":            "destination",
			} {
				writeFile(t, filepath.Join(root, path), contents)
			}
			before := snapshotFiles(t, root, "")
			entryID := scannedEntryID(t, root, "source/Example_9999999_P.pak")

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

// Covers unsafe folder names at the mutation boundary instead of relying on UI validation.
func TestCreateAndRenameFolderRejectInvalidNamesWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(root string) error
	}{
		{
			name: "create traversal name",
			run: func(root string) error {
				_, err := createFolder(root, "", "../outside")
				return err
			},
		},
		{
			name: "create reserved device name",
			run: func(root string) error {
				_, err := createFolder(root, "", "NUL")
				return err
			},
		},
		{
			name: "rename invalid name",
			run: func(root string) error {
				_, err := renameFolder(root, "source", "invalid:name")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "source", ".keep"), "source")
			before := snapshotFiles(t, root, "")

			// Act
			err := test.run(root)

			// Assert
			if err == nil {
				t.Fatal("folder operation succeeded, want validation error")
			}
			if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
				t.Errorf("rejected folder name changed files\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

// Proves the destination parent is revalidated when it changes into a link
// after the scanner has accepted it as a physical folder.
func TestCreateFolderRejectsDirectoryLinkIntroducedAfterScanning(t *testing.T) {
	// Arrange
	root := t.TempDir()
	externalRoot := t.TempDir()
	parentPath := filepath.Join(root, "parent")
	if err := os.Mkdir(parentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parentPath, filepath.Join(root, "parent-real")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalRoot, parentPath); err != nil {
		t.Skipf("directory symlinks are unavailable in this environment: %v", err)
	}
	beforeRoot := snapshotFiles(t, root, "")
	beforeExternal := snapshotFiles(t, externalRoot, "")

	// Act
	_, err = createFolderFromLibrary(library, "parent", "child", requireDirectory)

	// Assert
	if err == nil {
		t.Fatal("createFolderFromLibrary succeeded below a replacement directory link")
	}
	if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(beforeRoot, after) {
		t.Errorf("rejected linked parent changed root\nbefore: %#v\nafter: %#v", beforeRoot, after)
	}
	if after := snapshotFiles(t, externalRoot, ""); !reflect.DeepEqual(beforeExternal, after) {
		t.Errorf("rejected linked parent changed external directory\nbefore: %#v\nafter: %#v", beforeExternal, after)
	}
}

// Verifies bundle moves revalidate directory ancestry after their plan has
// been built, rather than trusting the directory state observed by scanning.
func TestBundlePlanRejectsDirectoryLinkIntroducedAfterPlanning(t *testing.T) {
	// Arrange
	root := t.TempDir()
	externalRoot := t.TempDir()
	writeFile(t, filepath.Join(root, "source", "Example_9999999_P.pak"), "primary")
	if err := os.Mkdir(filepath.Join(root, "destination"), 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := findEntry(library.Entries, scannedEntryID(t, root, "source/Example_9999999_P.pak"))
	if err != nil {
		t.Fatal(err)
	}
	stem, err := movedStem(entry, "destination")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildBundlePlan(root, entry, stem)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "source"), filepath.Join(root, "source-real")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalRoot, filepath.Join(root, "source")); err != nil {
		t.Skipf("directory symlinks are unavailable in this environment: %v", err)
	}

	// Act
	err = requireBundleDirectoryAncestry(root, plan)

	// Assert
	if err == nil {
		t.Fatal("requireBundleDirectoryAncestry accepted a replacement directory link")
	}
	if _, statErr := os.Lstat(filepath.Join(root, "destination", "Example_9999999_P.pak")); !os.IsNotExist(statErr) {
		t.Errorf("ancestry rejection changed destination: %v", statErr)
	}
}

// Exercises a later bundle-move failure and confirms rollback restores every
// moved member before the operation reports its failure.
func TestMoveModRollsBackAfterSidecarFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for path, contents := range map[string]string{
		"source/Example_9999999_P.pak":  "primary",
		"source/Example_9999999_P.utoc": "utoc",
		"source/Example_9999999_P.ucas": "ucas",
		"destination/.keep":             "folder marker",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	before := snapshotFiles(t, root, "")
	entryID := scannedEntryID(t, root, "source/Example_9999999_P.pak")
	move := func(source, destination string) error {
		if strings.HasSuffix(destination, ".utoc") {
			return errors.New("injected utoc move failure")
		}
		return moveFileWithoutReplace(source, destination)
	}

	// Act
	_, err := moveModWithMove(root, entryID, "destination", move)

	// Assert
	if err == nil {
		t.Fatal("moveModWithMove succeeded, want injected failure")
	}
	if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
		t.Errorf("rollback did not restore moved bundle\nbefore: %#v\nafter: %#v", before, after)
	}
}

// Confirms folder failures report source and destination state after the
// native move rejects the operation.
func TestMoveFolderReconcilesInjectedFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "source", ".keep"), "source")
	if err := os.Mkdir(filepath.Join(root, "destination"), 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	move := func(source, destination string) error {
		return errors.New("injected directory move failure")
	}

	// Act
	_, err = moveFolderWithMove(root, library.Folders, "source", "destination/source", move)

	// Assert
	if err == nil {
		t.Fatal("moveFolderWithMove succeeded, want injected failure")
	}
	if !strings.Contains(err.Error(), `"source" exists`) || !strings.Contains(err.Error(), `"destination/source" is missing`) {
		t.Errorf("error = %q, want reconciled folder states", err)
	}
	assertFileContents(t, filepath.Join(root, "source", ".keep"), "source")
	if _, statErr := os.Lstat(filepath.Join(root, "destination", "source")); !os.IsNotExist(statErr) {
		t.Errorf("failed move created destination folder: %v", statErr)
	}
}

// Confirms a post-move validation failure keeps its actual cause while
// reconciliation reports the source and destination directories.
func TestMoveFolderReconcilesPostMoveValidationFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "source", ".keep"), "source")
	if err := os.Mkdir(filepath.Join(root, "destination"), 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	validationFailure := errors.New("injected moved-folder validation failure")
	checkDirectory := func(path, label string) error {
		if label == "moved folder" {
			return validationFailure
		}
		return requireDirectory(path, label)
	}

	// Act
	_, err = moveFolderWithFunctions(
		root,
		library.Folders,
		"source",
		"destination/source",
		moveFileWithoutReplace,
		checkDirectory,
	)

	// Assert
	if !errors.Is(err, validationFailure) {
		t.Fatalf("moveFolderWithFunctions error = %v, want validation failure", err)
	}
	if !strings.Contains(err.Error(), `"source" is missing`) || !strings.Contains(err.Error(), `"destination/source" exists`) {
		t.Errorf("error = %q, want reconciled post-move states", err)
	}
	assertFileContents(t, filepath.Join(root, "destination", "source", ".keep"), "source")
}
