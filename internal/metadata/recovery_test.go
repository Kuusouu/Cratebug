package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadRecoversFromACorruptedPrimaryUsingTheBackup(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	good := Document{Settings: Settings{ModRoot: `C:\Mods`}}
	// Save twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	doc, recovery := store.Load()

	// Assert
	if !recovery.Recovered || recovery.Cause == nil {
		t.Fatalf("Recovery = %#v, want Recovered = true with a cause", recovery)
	}
	good.SchemaVersion = CurrentSchemaVersion
	if !reflect.DeepEqual(doc, good) {
		t.Errorf("Load() = %#v, want the backed-up document %#v", doc, good)
	}
	if _, err := os.Stat(path + quarantineSuffix); err != nil {
		t.Errorf("corrupted primary was not quarantined: %v", err)
	}
}

func TestLoadRecoversFromATruncatedPrimaryWrite(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	good := Document{Settings: Settings{ModRoot: `C:\Mods`}}
	// Save twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, full[:len(full)/2], 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	doc, recovery := store.Load()

	// Assert
	if !recovery.Recovered {
		t.Fatalf("Recovery = %#v, want Recovered = true for a truncated file", recovery)
	}
	good.SchemaVersion = CurrentSchemaVersion
	if !reflect.DeepEqual(doc, good) {
		t.Errorf("Load() = %#v, want the backed-up document %#v", doc, good)
	}
}

func TestLoadRejectsAnUnsupportedFutureSchemaVersionAndRecoversFromBackup(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	good := Document{Settings: Settings{ModRoot: `C:\Mods`}}
	// Save twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
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

	// Act
	doc, recovery := store.Load()

	// Assert
	if !recovery.Recovered {
		t.Fatalf("Recovery = %#v, want Recovered = true for an unsupported schema version", recovery)
	}
	good.SchemaVersion = CurrentSchemaVersion
	if !reflect.DeepEqual(doc, good) {
		t.Errorf("Load() = %#v, want the backed-up document %#v", doc, good)
	}
}

func TestLoadFallsBackToAFreshDocumentWhenPrimaryAndBackupAreBothUnusable(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	doc, recovery := store.Load()

	// Assert
	if !recovery.Recovered {
		t.Fatalf("Recovery = %#v, want Recovered = true even with no usable backup", recovery)
	}
	want := Document{SchemaVersion: CurrentSchemaVersion}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("Load() = %#v, want a fresh document %#v", doc, want)
	}
}

func TestLoadRecoveryDoesNotDiscardUnrelatedValidMetadata(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := NewStore(path)
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}
	// Save twice: the backup only holds what the primary held before its
	// most recent write, so a single save leaves no backup to recover from.
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	recovered, recovery := store.Load()

	// Assert
	if !recovery.Recovered {
		t.Fatal("Recovery.Recovered = false, want true")
	}
	if len(recovered.Tags) != 1 || recovered.Tags[0] != tag {
		t.Errorf("recovered tag catalog = %#v, want [%#v]", recovered.Tags, tag)
	}
	if !containsString(recovered.Mods[modID].Tags, tag.ID) {
		t.Errorf("recovered mod record lost its tag assignment: %#v", recovered.Mods[modID])
	}
}

func TestOrphanedModsReportsRecordsAbsentFromALiveScan(t *testing.T) {
	// Arrange
	var doc Document
	stillPresent, err := doc.EnsureMod("mod:folder:present")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := doc.EnsureMod("mod:folder:deleted")
	if err != nil {
		t.Fatal(err)
	}
	liveScannerIDs := map[string]struct{}{"mod:folder:present": {}}

	// Act
	orphaned := doc.OrphanedMods(liveScannerIDs)

	// Assert
	if _, present := orphaned[stillPresent]; present {
		t.Error("a mod present in the live scan was reported as orphaned")
	}
	if _, present := orphaned[gone]; !present {
		t.Error("a mod absent from the live scan was not reported as orphaned")
	}
}
