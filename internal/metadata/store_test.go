package metadata

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadWithNoSavedFileReturnsFreshDocument(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))

	// Act
	doc, recovery := store.Load()

	// Assert
	want := Document{SchemaVersion: CurrentSchemaVersion}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("Load() = %#v, want %#v", doc, want)
	}
	if recovery.Recovered {
		t.Errorf("Recovery = %#v, want Recovered = false for a first launch", recovery)
	}
}

func TestSaveThenLoadRoundTripsExactly(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	written := Document{Settings: Settings{ModRoot: `C:\Mods`}}

	// Act
	if err := store.Save(written); err != nil {
		t.Fatal(err)
	}
	read, _ := store.Load()

	// Assert
	written.SchemaVersion = CurrentSchemaVersion
	if !reflect.DeepEqual(read, written) {
		t.Errorf("Load() = %#v, want %#v", read, written)
	}
}

func TestSaveStampsCurrentSchemaVersion(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))

	// Act
	if err := store.Save(Document{SchemaVersion: 0}); err != nil {
		t.Fatal(err)
	}
	doc, _ := store.Load()

	// Assert
	if doc.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", doc.SchemaVersion, CurrentSchemaVersion)
	}
}

// Confirms Save maintains a last-known-good backup, which corrupt-file
// recovery (task 5.4) will read when the primary file cannot be parsed.
func TestSaveKeepsPreviousContentAsBackup(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	first := Document{Settings: Settings{ModRoot: `C:\First`}}
	second := Document{Settings: Settings{ModRoot: `C:\Second`}}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}

	// Assert
	backupDoc, _ := NewStore(path + backupSuffix).Load()
	first.SchemaVersion = CurrentSchemaVersion
	if !reflect.DeepEqual(backupDoc, first) {
		t.Errorf("backup document = %#v, want the previously saved document %#v", backupDoc, first)
	}
}

// Proves the atomic-write mechanism Save relies on: a failure before the
// final rename must never modify a destination file that already exists.
func TestWriteFileAtomicallyLeavesDestinationUntouchedOnFailure(t *testing.T) {
	// Arrange
	destination := filepath.Join(t.TempDir(), "metadata.json")
	original := []byte("previous valid content")
	if err := os.WriteFile(destination, original, 0o600); err != nil {
		t.Fatal(err)
	}
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")

	// Act
	err := writeFileAtomically(missingDir, destination, []byte("new content that must not land"))

	// Assert
	if err == nil {
		t.Fatal("writeFileAtomically() succeeded, want an error from the missing temp directory")
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Errorf("destination content = %q, want %q (unchanged)", got, original)
	}
}
