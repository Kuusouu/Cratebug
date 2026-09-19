package reveal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

func TestResolveFolderOpensLibraryRootAndNestedFolder(t *testing.T) {
	// Arrange
	root := t.TempDir()
	nested := filepath.Join(root, "Characters", "Hulk")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}

	// Act
	library, err := ResolveFolder(root, "")
	if err != nil {
		t.Fatal(err)
	}
	folder, err := ResolveFolder(root, "Characters/Hulk")
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if library.Path != wantRoot || library.SelectItem {
		t.Errorf("library root target = %#v, want path %q unselected", library, wantRoot)
	}
	wantFolder := filepath.Join(wantRoot, "Characters", "Hulk")
	if folder.Path != wantFolder || folder.SelectItem {
		t.Errorf("nested folder target = %#v, want path %q unselected", folder, wantFolder)
	}
}

func TestResolveFolderRejectsUnsafeOrMissingPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "not-a-folder"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name    string
		modRoot string
		folder  string
		want    string
	}{
		{name: "empty root", modRoot: "", folder: "", want: "mod library folder is not set"},
		{name: "whitespace root", modRoot: "   ", folder: "", want: "mod library folder is not set"},
		{name: "traversal", modRoot: root, folder: "../outside", want: "outside the mod root"},
		{name: "absolute", modRoot: root, folder: root, want: "outside the mod root"},
		{name: "missing", modRoot: root, folder: "missing", want: "folder is missing"},
		{name: "file", modRoot: root, folder: "not-a-folder", want: "not a directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange / Act
			_, err := ResolveFolder(test.modRoot, test.folder)

			// Assert
			if err == nil {
				t.Fatal("ResolveFolder succeeded, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q, want substring %q", err, test.want)
			}
		})
	}
}

func TestResolveModSelectsPrimaryAndOrphanSidecar(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Characters", "Example_9999999_P.pak"), "primary")
	writeFile(t, filepath.Join(root, "orphan", "Orphan_9999999_P.utoc"), "utoc")
	writeFile(t, filepath.Join(root, "orphan", "Orphan_9999999_P.ucas"), "ucas")

	primaryID := scannedEntryID(t, root, "Characters/Example_9999999_P.pak")
	orphanID := scannedOrphanID(t, root)

	// Act
	primary, err := ResolveMod(root, primaryID)
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := ResolveMod(root, orphanID)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	wantPrimary := filepath.Join(wantRoot, "Characters", "Example_9999999_P.pak")
	if primary.Path != wantPrimary || !primary.SelectItem {
		t.Errorf("primary target = %#v, want selected %q", primary, wantPrimary)
	}
	wantOrphan := filepath.Join(wantRoot, "orphan", "Orphan_9999999_P.utoc")
	if orphan.Path != wantOrphan || !orphan.SelectItem {
		t.Errorf("orphan target = %#v, want selected %q", orphan, wantOrphan)
	}
}

func TestResolveModRejectsUnknownAndMissingEntries(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Example_9999999_P.pak"), "primary")

	for _, test := range []struct {
		name    string
		modRoot string
		entryID string
		want    string
	}{
		{name: "unknown id", modRoot: root, entryID: "mod::missing", want: "not present in the current scan"},
		{name: "empty root", modRoot: "", entryID: "mod::example", want: "stat mod root"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Act
			_, err := ResolveMod(test.modRoot, test.entryID)

			// Assert
			if err == nil {
				t.Fatal("ResolveMod succeeded, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q, want substring %q", err, test.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func scannedEntryID(t *testing.T, root, primaryPath string) string {
	t.Helper()
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range library.Entries {
		if entry.PrimaryPath == primaryPath {
			return entry.ID
		}
	}
	t.Fatalf("scanner did not return primary %q", primaryPath)
	return ""
}

func scannedOrphanID(t *testing.T, root string) string {
	t.Helper()
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range library.Entries {
		if entry.Kind == discovery.EntryOrphanedSidecar {
			return entry.ID
		}
	}
	t.Fatal("scanner did not return an orphaned sidecar")
	return ""
}
