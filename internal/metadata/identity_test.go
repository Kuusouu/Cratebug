package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type staticGameRunningChecker struct{}

func (staticGameRunningChecker) IsGameRunning() (bool, error) {
	return false, nil
}

func writeFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func scanEntryID(t *testing.T, root, relativePath string) string {
	t.Helper()
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range library.Entries {
		if entry.PrimaryPath == filepath.ToSlash(relativePath) {
			return entry.ID
		}
	}
	t.Fatalf("no scanned entry for %q", relativePath)
	return ""
}

func TestEnsureModReturnsTheSameIdentityForTheSameScannerID(t *testing.T) {
	// Arrange
	var doc Document
	scannerID := "mod:folder:example"

	// Act
	first, err := doc.EnsureMod(scannerID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := doc.EnsureMod(scannerID)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if first != second {
		t.Errorf("EnsureMod() returned %q then %q, want the same identity", first, second)
	}
}

func TestEnsureModGivesSameNamedModsInDifferentFoldersDistinctIdentities(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "FolderA", "SharedStem_9999999_P.pak"))
	writeFixture(t, filepath.Join(root, "FolderB", "SharedStem_9999999_P.pak"))
	var doc Document

	// Act
	idA, err := doc.EnsureMod(scanEntryID(t, root, "FolderA/SharedStem_9999999_P.pak"))
	if err != nil {
		t.Fatal(err)
	}
	idB, err := doc.EnsureMod(scanEntryID(t, root, "FolderB/SharedStem_9999999_P.pak"))
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if idA == idB {
		t.Errorf("same-named mods in different folders got the same identity: %q", idA)
	}
}

func TestReconcileModReturnsFalseForAnUnknownScannerID(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	reconciled := doc.ReconcileMod("mod:folder:unknown", "mod:folder:renamed")

	// Assert
	if reconciled {
		t.Error("ReconcileMod() = true, want false for an untracked scanner ID")
	}
}

// Confirms a real bundle rename, priority change, and move each keep the same
// persistent identity when reconciled through the Result the mutation
// package already returns, matching how App would drive this in production.
func TestPersistentIdentitySurvivesRealMutations(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(t *testing.T, executor mutation.Executor, root, entryID string) mutation.Result
	}{
		{
			name: "rename",
			apply: func(t *testing.T, executor mutation.Executor, root, entryID string) mutation.Result {
				t.Helper()
				result, err := executor.Execute(mutation.NewRenameModOperation(root, entryID, "Renamed"))
				if err != nil {
					t.Fatal(err)
				}
				return result
			},
		},
		{
			name: "priority",
			apply: func(t *testing.T, executor mutation.Executor, root, entryID string) mutation.Result {
				t.Helper()
				result, err := executor.Execute(mutation.NewSetPriorityOperation(root, entryID, 3))
				if err != nil {
					t.Fatal(err)
				}
				return result
			},
		},
		{
			name: "move",
			apply: func(t *testing.T, executor mutation.Executor, root, entryID string) mutation.Result {
				t.Helper()
				if err := os.Mkdir(filepath.Join(root, "Destination"), 0o700); err != nil {
					t.Fatal(err)
				}
				result, err := executor.Execute(mutation.NewMoveModOperation(root, entryID, "Destination"))
				if err != nil {
					t.Fatal(err)
				}
				return result
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, "Example_9999999_P.pak"))
			entryID := scanEntryID(t, root, "Example_9999999_P.pak")
			var doc Document
			identity, err := doc.EnsureMod(entryID)
			if err != nil {
				t.Fatal(err)
			}
			executor := mutation.NewExecutor(staticGameRunningChecker{})

			// Act
			result := test.apply(t, executor, root, entryID)
			reconciled := doc.ReconcileMod(result.PreviousID, result.ID)

			// Assert
			if !reconciled {
				t.Fatal("ReconcileMod() = false, want true")
			}
			afterID, ok := doc.FindModByScannerID(result.ID)
			if !ok || afterID != identity {
				t.Errorf("FindModByScannerID(new) = (%q, %v), want (%q, true)", afterID, ok, identity)
			}
			if _, ok := doc.FindModByScannerID(entryID); ok && entryID != result.ID {
				t.Errorf("old scanner ID %q still resolves after %s", entryID, test.name)
			}
		})
	}
}

func TestSetModNexusSourceRejectsAnUnknownMod(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetModNexusSource("mod-missing", 1, 2, "1.0")

	// Assert
	if err == nil {
		t.Fatal("SetModNexusSource() succeeded for an unknown mod, want an error")
	}
}

func TestSetModNexusSourceRejectsNegativeIDs(t *testing.T) {
	for _, test := range []struct {
		name        string
		nexusModID  int
		nexusFileID int
	}{
		{name: "negative mod ID", nexusModID: -1, nexusFileID: 2},
		{name: "negative file ID", nexusModID: 1, nexusFileID: -2},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			var doc Document
			modID, err := doc.EnsureMod("mod:folder:example")
			if err != nil {
				t.Fatal(err)
			}

			// Act
			err = doc.SetModNexusSource(modID, test.nexusModID, test.nexusFileID, "1.0")

			// Assert
			if err == nil {
				t.Fatal("SetModNexusSource() succeeded for a negative ID, want an error")
			}
			record := doc.Mods[modID]
			if record.NexusModID != 0 || record.NexusFileID != 0 || record.NexusVersion != "" {
				t.Errorf("Nexus fields were mutated on reject: {%d, %d, %q}", record.NexusModID, record.NexusFileID, record.NexusVersion)
			}
		})
	}
}

func TestSetModNexusSourceAssignsOntoTheExistingRecord(t *testing.T) {
	// Arrange
	var doc Document
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	err = doc.SetModNexusSource(modID, 123, 456, "1.2.3")

	// Assert
	if err != nil {
		t.Fatalf("SetModNexusSource() = %v, want no error", err)
	}
	record := doc.Mods[modID]
	if record.ScannerID != "mod:folder:example" {
		t.Errorf("ScannerID = %q, want %q", record.ScannerID, "mod:folder:example")
	}
	if len(record.Tags) != 1 || record.Tags[0] != tag.ID {
		t.Errorf("Tags = %#v, want [%q]", record.Tags, tag.ID)
	}
	if record.NexusModID != 123 || record.NexusFileID != 456 || record.NexusVersion != "1.2.3" {
		t.Errorf("Nexus fields = {%d, %d, %q}, want {123, 456, \"1.2.3\"}", record.NexusModID, record.NexusFileID, record.NexusVersion)
	}
}

func TestSetModNexusSourceAcceptsZerosToClear(t *testing.T) {
	// Arrange
	var doc Document
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetModNexusSource(modID, 123, 456, "1.2.3"); err != nil {
		t.Fatal(err)
	}

	// Act
	err = doc.SetModNexusSource(modID, 0, 0, "")

	// Assert
	if err != nil {
		t.Fatalf("SetModNexusSource() = %v, want no error", err)
	}
	record := doc.Mods[modID]
	if record.NexusModID != 0 || record.NexusFileID != 0 || record.NexusVersion != "" {
		t.Errorf("Nexus fields = {%d, %d, %q}, want zeros", record.NexusModID, record.NexusFileID, record.NexusVersion)
	}
}

func TestModNexusSourceSurvivesASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetModNexusSource(modID, 123, 456, "1.2.3"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	record, ok := reloaded.Mods[modID]
	if !ok {
		t.Fatal("mod record missing after load")
	}
	if record.NexusModID != 123 || record.NexusFileID != 456 || record.NexusVersion != "1.2.3" {
		t.Errorf("Nexus fields = {%d, %d, %q}, want {123, 456, \"1.2.3\"}", record.NexusModID, record.NexusFileID, record.NexusVersion)
	}
}

func TestEnsureModRecordLoadsWithZeroNexusFields(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	if _, err := doc.EnsureMod("mod:folder:example"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	modID, ok := reloaded.FindModByScannerID("mod:folder:example")
	if !ok {
		t.Fatal("EnsureMod record missing after load")
	}
	record := reloaded.Mods[modID]
	if record.NexusModID != 0 || record.NexusFileID != 0 || record.NexusVersion != "" {
		t.Errorf("Nexus fields = {%d, %d, %q}, want zeros", record.NexusModID, record.NexusFileID, record.NexusVersion)
	}
}

func TestDocumentWhoseModsHaveNoNexusFieldsLoadsAsZeros(t *testing.T) {
	// Arrange: a schema-1 document written before these fields existed has
	// no nexus keys on the mod record. CurrentSchemaVersion stays 1.
	path := filepath.Join(t.TempDir(), "metadata.json")
	raw := `{"schemaVersion": 1, "settings": {}, "mods": {"mod-1": {"scannerID": "mod:folder:example"}}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	// Act
	reloaded, recovery := store.Load()

	// Assert
	if recovery.Recovered {
		t.Fatalf("Recovery = %#v, want Recovered = false for a schema-1 document", recovery)
	}
	record, ok := reloaded.Mods["mod-1"]
	if !ok {
		t.Fatal("mod-1 missing after load")
	}
	if record.NexusModID != 0 || record.NexusFileID != 0 || record.NexusVersion != "" {
		t.Errorf("Nexus fields = {%d, %d, %q}, want zeros", record.NexusModID, record.NexusFileID, record.NexusVersion)
	}
}
