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
