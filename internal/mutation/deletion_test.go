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

// Verifies deletion sends the complete IoStore bundle through the injected
// Recycle Bin boundary and reports the primary as deleted only after rescan.
func TestDeleteModRecyclesCompleteIoStoreBundle(t *testing.T) {
	// Arrange
	root := t.TempDir()
	recycleRoot := t.TempDir()
	for path, contents := range map[string]string{
		"Example_9999999_P.pak":  "primary",
		"Example_9999999_P.utoc": "utoc",
		"Example_9999999_P.ucas": "ucas",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	entryID := scannedEntryID(t, root, "Example_9999999_P.pak")
	recycle := moveToDisposableRecycleBin(t, recycleRoot)

	// Act
	result, err := deleteModWithRecycle(root, entryID, true, recycle)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !result.Deleted || result.PreviousID != entryID || result.PreviousPrimaryPath != "Example_9999999_P.pak" {
		t.Errorf("result = %#v, want deleted original bundle result", result)
	}
	if got := snapshotFiles(t, root, ""); len(got) != 0 {
		t.Errorf("mod root still contains files: %#v", got)
	}
	want := map[string]string{
		"Example_9999999_P.pak":  "primary",
		"Example_9999999_P.utoc": "utoc",
		"Example_9999999_P.ucas": "ucas",
	}
	if got := snapshotFiles(t, recycleRoot, ""); !reflect.DeepEqual(got, want) {
		t.Errorf("disposable Recycle Bin = %#v, want %#v", got, want)
	}
}

// Allows an incomplete bundle to be removed when every present member still
// belongs unambiguously to the scanner-discovered mod.
func TestDeleteModRecyclesIncompleteIoStoreBundle(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
	}{
		{
			name: "missing ucas",
			files: map[string]string{
				"Example_9999999_P.pak":  "primary",
				"Example_9999999_P.utoc": "utoc",
			},
		},
		{
			name: "missing utoc",
			files: map[string]string{
				"Example_9999999_P.pak":  "primary",
				"Example_9999999_P.ucas": "ucas",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			recycleRoot := t.TempDir()
			for path, contents := range test.files {
				writeFile(t, filepath.Join(root, path), contents)
			}
			entryID := scannedEntryID(t, root, "Example_9999999_P.pak")

			// Act
			result, err := deleteModWithRecycle(root, entryID, true, moveToDisposableRecycleBin(t, recycleRoot))
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			if !result.Deleted {
				t.Errorf("result = %#v, want deleted bundle result", result)
			}
			if got := snapshotFiles(t, root, ""); len(got) != 0 {
				t.Errorf("mod root still contains files: %#v", got)
			}
			if got := snapshotFiles(t, recycleRoot, ""); !reflect.DeepEqual(got, test.files) {
				t.Errorf("disposable Recycle Bin = %#v, want %#v", got, test.files)
			}
		})
	}
}

// Ensures a parent directory that becomes a link after planning cannot redirect
// the shell operation to a matching file outside the mod root.
func TestDeletionPlanRejectsDirectoryLinkIntroducedAfterPlanning(t *testing.T) {
	// Arrange
	root := t.TempDir()
	externalRoot := t.TempDir()
	primaryPath := filepath.Join(root, "source", "Example_9999999_P.pak")
	writeFile(t, primaryPath, "primary")
	writeFile(t, filepath.Join(externalRoot, "Example_9999999_P.pak"), "external")

	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := findEntry(library.Entries, scannedEntryID(t, root, "source/Example_9999999_P.pak"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildDeletionPlan(library.Root, entry)
	if err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(root, "source")
	if err := os.Rename(sourcePath, filepath.Join(root, "source-real")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalRoot, sourcePath); err != nil {
		t.Skipf("directory symlinks are unavailable in this environment: %v", err)
	}
	recycleCalled := false

	// Act
	err = plan.apply(library.Root, func(paths []string) error {
		recycleCalled = true
		return nil
	})

	// Assert
	if err == nil {
		t.Fatal("plan.apply succeeded through a replacement directory link")
	}
	if recycleCalled {
		t.Fatal("plan.apply called the Recycle Bin boundary through a replacement directory link")
	}
	assertFileContents(t, filepath.Join(externalRoot, "Example_9999999_P.pak"), "external")
	assertFileContents(t, filepath.Join(root, "source-real", "Example_9999999_P.pak"), "primary")
}

// Ensures missing confirmation and unsafe scanner entries leave fixtures intact.
func TestDeleteModRejectsUnsafeRequestsWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name      string
		files     []string
		confirmed bool
		entryPath string
	}{
		{name: "missing confirmation", files: []string{"Example_9999999_P.pak"}, entryPath: "Example_9999999_P.pak"},
		{name: "ambiguous primary", files: []string{"Example_9999999_P.pak", "Example_9999999_P.pak_crateoff"}, confirmed: true, entryPath: "Example_9999999_P.pak"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, filepath.Join(root, file), file)
			}
			before := snapshotFiles(t, root, "")
			entryID := scannedEntryID(t, root, test.entryPath)
			recycleCalled := false

			// Act
			_, err := deleteModWithRecycle(root, entryID, test.confirmed, func(paths []string) error {
				recycleCalled = true
				return nil
			})

			// Assert
			if err == nil {
				t.Fatal("deleteModWithRecycle succeeded, want rejection")
			}
			if recycleCalled {
				t.Fatal("rejected deletion called the Recycle Bin boundary")
			}
			if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
				t.Errorf("rejected deletion changed files\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

// Reports actual remaining files when the platform boundary partially recycles a bundle.
func TestDeleteModReconcilesPartialRecycleFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	recycleRoot := t.TempDir()
	for path, contents := range map[string]string{
		"Example_9999999_P.pak":  "primary",
		"Example_9999999_P.utoc": "utoc",
		"Example_9999999_P.ucas": "ucas",
	} {
		writeFile(t, filepath.Join(root, path), contents)
	}
	entryID := scannedEntryID(t, root, "Example_9999999_P.pak")
	recycle := func(paths []string) error {
		if err := os.Rename(paths[0], filepath.Join(recycleRoot, filepath.Base(paths[0]))); err != nil {
			return err
		}
		return errors.New("injected Recycle Bin failure")
	}

	// Act
	_, err := deleteModWithRecycle(root, entryID, true, recycle)

	// Assert
	if err == nil {
		t.Fatal("deleteModWithRecycle succeeded, want partial failure")
	}
	if !strings.Contains(err.Error(), "Example_9999999_P.pak") || !strings.Contains(err.Error(), "Example_9999999_P.utoc") {
		t.Errorf("error = %q, want reconciled partial bundle state", err)
	}
	assertFileContents(t, filepath.Join(recycleRoot, "Example_9999999_P.pak"), "primary")
	assertFileContents(t, filepath.Join(root, "Example_9999999_P.utoc"), "utoc")
	assertFileContents(t, filepath.Join(root, "Example_9999999_P.ucas"), "ucas")
}

// Guards the shell API's double-null-terminated multi-path requirement without
// calling the user's real Recycle Bin.
func TestJoinShellPaths(t *testing.T) {
	// Arrange
	paths := []string{"one.pak", "two.utoc"}

	// Act
	got := joinShellPaths(paths)

	// Assert
	want := "one.pak\x00two.utoc\x00\x00"
	if got != want {
		t.Errorf("joinShellPaths() = %q, want %q", got, want)
	}
}

func moveToDisposableRecycleBin(t *testing.T, recycleRoot string) func([]string) error {
	t.Helper()
	return func(paths []string) error {
		for _, path := range paths {
			if err := os.Rename(path, filepath.Join(recycleRoot, filepath.Base(path))); err != nil {
				return err
			}
		}
		return nil
	}
}
