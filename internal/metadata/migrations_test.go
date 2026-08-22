package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A file with no "schemaVersion" field represents schema version 0: content
// written before that field existed, or a hand-authored fixture. Load must
// migrate it forward to CurrentSchemaVersion rather than rejecting it.
func TestLoadMigratesAnUnversionedFileToTheCurrentSchema(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	unversioned, err := json.Marshal(map[string]any{
		"settings": map[string]any{"modRoot": `C:\Mods`},
		"tags":     []map[string]any{{"id": "tag-1", "name": "Combat"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, unversioned, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	// Act
	doc, recovery := store.Load()

	// Assert
	if recovery.Recovered {
		t.Errorf("Recovery = %#v, want Recovered = false for a successful migration", recovery)
	}
	want := Document{
		SchemaVersion: CurrentSchemaVersion,
		Settings:      Settings{ModRoot: `C:\Mods`},
		Tags:          []Tag{{ID: "tag-1", Name: "Combat"}},
	}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("Load() = %#v, want %#v", doc, want)
	}
}

func TestMigratedDocumentRoundTripsAfterBeingSaved(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	unversioned, err := json.Marshal(map[string]any{"settings": map[string]any{"modRoot": `C:\Mods`}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, unversioned, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	migrated, _ := store.Load()

	// Act
	if err := store.Save(migrated); err != nil {
		t.Fatal(err)
	}
	reloaded, recovery := store.Load()

	// Assert
	if recovery.Recovered {
		t.Errorf("Recovery = %#v, want Recovered = false after saving the migrated document", recovery)
	}
	if !reflect.DeepEqual(reloaded, migrated) {
		t.Errorf("Load() after save = %#v, want %#v", reloaded, migrated)
	}
}

func TestReadDocumentRejectsAVersionOlderThanAnyRegisteredMigration(t *testing.T) {
	// Arrange: schema version -1 does not correspond to any real Cratebug
	// release; it only exercises the guard for a version this build's
	// migration table has no step for, which a version that old would hit.
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	if err := os.WriteFile(path, []byte(`{"schemaVersion": -1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := store.readDocument(path)

	// Assert
	if err == nil {
		t.Fatal("readDocument() succeeded, want an error for an unmigratable schema version")
	}
}

// Confirms rejecting an unsupported future schema version never rewrites or
// otherwise alters the original file; it is only moved aside intact.
func TestLoadQuarantinesAnUnsupportedFutureVersionWithoutAlteringItsContent(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	future, err := json.Marshal(map[string]any{
		"schemaVersion": 999999,
		"settings":      map[string]any{"modRoot": `C:\FromTheFuture`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, future, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	// Act
	_, recovery := store.Load()

	// Assert
	if !recovery.Recovered {
		t.Fatal("Recovery.Recovered = false, want true")
	}
	if recovery.Cause == nil {
		t.Fatal("Recovery.Cause is nil, want an explanation of the unsupported version")
	}
	quarantined, err := os.ReadFile(path + quarantineSuffix)
	if err != nil {
		t.Fatalf("quarantined file is unreadable: %v", err)
	}
	if !reflect.DeepEqual(quarantined, future) {
		t.Errorf("quarantined content = %q, want the original untouched content %q", quarantined, future)
	}
}
