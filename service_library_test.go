package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/conflict"
	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/modtype"
)

type staticDetectionProvider struct {
	name      string
	detection gamedetect.Detection
}

func (provider staticDetectionProvider) Name() string {
	return provider.name
}

func (provider staticDetectionProvider) Detect() (gamedetect.Detection, error) {
	return provider.detection, nil
}

func TestScanLibrary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := testApp(t, false)

	// Act
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if library.Root != root {
		t.Errorf("Root = %q, want %q", library.Root, root)
	}
	if len(library.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(library.Entries))
	}
	entry := library.Entries[0]
	if entry.DisplayName != "Example" {
		t.Errorf("DisplayName = %q, want %q", entry.DisplayName, "Example")
	}
	if entry.State != discovery.StateEnabled {
		t.Errorf("State = %q, want %q", entry.State, discovery.StateEnabled)
	}
}

func TestOpenInExplorerRejectsMissingTargets(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	root := t.TempDir()

	// Act
	folderErr := app.library.OpenFolderInExplorer(root, "missing")
	modErr := app.library.OpenModInExplorer(root, "mod::missing")

	// Assert
	if folderErr == nil {
		t.Fatal("OpenFolderInExplorer succeeded, want error")
	}
	if modErr == nil {
		t.Fatal("OpenModInExplorer succeeded, want error")
	}
}

func TestSetModRootPersistsAcrossLoads(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	if err := app.library.SetModRoot(`C:\Mods`); err != nil {
		t.Fatal(err)
	}
	state := app.settings.LoadMetadata()

	// Assert
	if state.Document.Settings.ModRoot != `C:\Mods` {
		t.Errorf("Settings.ModRoot = %q, want %q", state.Document.Settings.ModRoot, `C:\Mods`)
	}
}

func TestDetectLibraryDispatchesToTheNamedProvider(t *testing.T) {
	// Arrange
	want := gamedetect.Detection{State: gamedetect.StateLibraryFound, LibraryPath: `C:\Mods\~mods`}
	app := testApp(t, false)
	app.detector = gamedetect.NewRegistry(staticDetectionProvider{name: "steam", detection: want})

	// Act
	detection, err := app.library.DetectLibrary("steam")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection != want {
		t.Errorf("DetectLibrary() = %+v, want %+v", detection, want)
	}
}

func TestDetectLibraryRejectsAnUnknownProvider(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	_, err := app.library.DetectLibrary("egs")

	// Assert
	if err == nil {
		t.Fatal("DetectLibrary() succeeded for an unknown provider, want an error")
	}
}

func TestCreateLibrarySurfacesTheCreatedPath(t *testing.T) {
	// Arrange
	paksPath := t.TempDir()
	app := testApp(t, false)
	app.detector = gamedetect.NewRegistry(staticDetectionProvider{
		name:      "steam",
		detection: gamedetect.Detection{State: gamedetect.StateInstallFound, PaksPath: paksPath},
	})

	// Act
	libraryPath, err := app.library.CreateLibrary("steam")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(paksPath, gamedetect.LibraryDirName); libraryPath != want {
		t.Errorf("CreateLibrary() = %q, want %q", libraryPath, want)
	}
	if _, err := os.Stat(libraryPath); err != nil {
		t.Errorf("the created mod library directory is missing: %v", err)
	}
}

func TestClassifyLibrary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	table := modtype.CharacterTable{
		CharacterNames: map[string]string{"1044": "Blade"},
	}
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t), nil, &table, nil, testSecretStore(t))
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	results, err := app.library.ClassifyLibrary(root, library.Entries)

	// Assert
	if err != nil {
		t.Fatalf("ClassifyLibrary() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	// Without a live worker running in this unit test, it degrades to CategoryUnknown gracefully
	if results[library.Entries[0].ID].Category != modtype.CategoryUnknown {
		t.Errorf("Category = %q, want CategoryUnknown", results[library.Entries[0].ID].Category)
	}
}

// A minimal modtype.PoolWorker double that answers list_pak with a canned
// response, so tests can exercise real classification (and therefore real
// path retention) without a live UAssetTool worker process.
type fakeConflictWorker struct {
	response string
	calls    int
}

func (w *fakeConflictWorker) Call(action string, _ map[string]any, result any) error {
	w.calls++
	if action != "list_pak" {
		return nil
	}
	return json.Unmarshal([]byte(w.response), result)
}

func (w *fakeConflictWorker) Alive() bool  { return true }
func (w *fakeConflictWorker) Close() error { return nil }

func writePakFixture(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDetectConflictsFindsSamePriorityGroupAndReusesCache(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writePakFixture(t, root, "ModA_9999999_P.pak")
	writePakFixture(t, root, "ModB_9999999_P.pak")

	worker := &fakeConflictWorker{response: `{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`}
	classifier := modtype.NewSessionClassifier(func() (modtype.PoolWorker, error) { return worker, nil })
	defer func() {
		if err := classifier.Close(); err != nil {
			t.Errorf("test cleanup: %v", err)
		}
	}()

	table := modtype.CharacterTable{}
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t), classifier, &table, nil, testSecretStore(t))
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	result, err := app.library.DetectConflicts(root, library.Entries)

	// Assert
	if err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("Groups = %+v, want exactly one group", result.Groups)
	}
	if result.Groups[0].Relationship != conflict.SamePriority {
		t.Errorf("Relationship = %q, want %q (both mods share the same trailing-nine priority)", result.Groups[0].Relationship, conflict.SamePriority)
	}
	if len(result.Unavailable) != 0 {
		t.Errorf("Unavailable = %v, want none", result.Unavailable)
	}
	callsAfterFirstScan := worker.calls

	// Act - second scan of the same unchanged library must not re-list either mod's contents
	if _, err := app.library.DetectConflicts(root, library.Entries); err != nil {
		t.Fatalf("second DetectConflicts() error = %v", err)
	}

	// Assert
	if worker.calls != callsAfterFirstScan {
		t.Errorf("worker.calls after second scan = %d, want %d (unchanged mods should hit the classification cache)", worker.calls, callsAfterFirstScan)
	}
}

func TestDetectConflictsReportsUnavailableWithoutALiveWorker(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writePakFixture(t, root, "ModA_9999999_P.pak")

	table := modtype.CharacterTable{}
	app := newApp(staticGameRunningChecker{}, testMetadataStore(t), nil, &table, nil, testSecretStore(t))
	library, err := app.library.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	result, err := app.library.DetectConflicts(root, library.Entries)

	// Assert
	if err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}
	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none", result.Groups)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != library.Entries[0].ID {
		t.Errorf("Unavailable = %v, want [%s]", result.Unavailable, library.Entries[0].ID)
	}
}
