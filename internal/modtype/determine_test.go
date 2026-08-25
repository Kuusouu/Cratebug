package modtype

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Records every call it received, in order, and answers with a per-action
// canned response or error, without needing a live worker process.
type fakeCaller struct {
	calls     []fakeCall
	responses map[string]string
	errs      map[string]error
}

type fakeCall struct {
	action string
	params map[string]any
}

func (f *fakeCaller) Call(action string, params map[string]any, result any) error {
	f.calls = append(f.calls, fakeCall{action: action, params: params})
	if err, ok := f.errs[action]; ok {
		return err
	}
	if body, ok := f.responses[action]; ok {
		return json.Unmarshal([]byte(body), result)
	}
	return nil
}

func (f *fakeCaller) actions() []string {
	actions := make([]string, len(f.calls))
	for i, call := range f.calls {
		actions[i] = call.action
	}
	return actions
}

func classicEntry(primaryPath string) discovery.Entry {
	return discovery.Entry{
		ID:           "classic-1",
		Kind:         discovery.EntryMod,
		BundleFormat: discovery.BundleFormatClassic,
		PrimaryPath:  primaryPath,
	}
}

func iostoreEntry(utocPath string) discovery.Entry {
	return discovery.Entry{
		ID:           "iostore-1",
		Kind:         discovery.EntryMod,
		BundleFormat: discovery.BundleFormatIoStore,
		Sidecars:     discovery.Sidecars{UTOC: utocPath},
	}
}

func TestDetermineRejectsNonModEntry(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}
	entry := discovery.Entry{ID: "orphan-1", Kind: discovery.EntryOrphanedSidecar}

	// Act
	_, err := Determine(fake, "root", entry)

	// Assert
	if err == nil {
		t.Fatal("Determine() error = nil, want an error for a non-mod entry")
	}
	if len(fake.calls) != 0 {
		t.Errorf("Determine() made %d calls, want 0", len(fake.calls))
	}
}

func TestDetermineClassifiesClassicMod(t *testing.T) {
	// Arrange
	fake := &fakeCaller{responses: map[string]string{
		"list_pak": `{"files":[{"path":"Characters/SK_Hero.uasset"}]}`,
	}}
	entry := classicEntry("Mods/Example.pak")

	// Act
	category, err := Determine(fake, "C:/root", entry)

	// Assert
	if err != nil {
		t.Fatalf("Determine() error = %v, want nil", err)
	}
	if category != CategoryMesh {
		t.Errorf("Determine() = %q, want %q", category, CategoryMesh)
	}
	if len(fake.calls) != 1 || fake.calls[0].action != "list_pak" {
		t.Fatalf("calls = %v, want exactly one list_pak call", fake.actions())
	}
	wantPath := filepath.Join("C:/root", "Mods", "Example.pak")
	if fake.calls[0].params["file_path"] != wantPath {
		t.Errorf("params[file_path] = %v, want %q", fake.calls[0].params["file_path"], wantPath)
	}
}

func TestDetermineClassifiesUnencryptedIoStoreModInOrder(t *testing.T) {
	// Arrange
	fake := &fakeCaller{responses: map[string]string{
		"is_iostore_encrypted": `{"encrypted":false}`,
		"list_iostore_files":   `{"files":["Characters/SK_Hero"]}`,
	}}
	entry := iostoreEntry("Mods/Example.utoc")

	// Act
	category, err := Determine(fake, "C:/root", entry)

	// Assert
	if err != nil {
		t.Fatalf("Determine() error = %v, want nil", err)
	}
	if category != CategoryMesh {
		t.Errorf("Determine() = %q, want %q", category, CategoryMesh)
	}
	wantOrder := []string{"is_iostore_encrypted", "list_iostore_files"}
	gotOrder := fake.actions()
	if len(gotOrder) != len(wantOrder) || gotOrder[0] != wantOrder[0] || gotOrder[1] != wantOrder[1] {
		t.Fatalf("call order = %v, want %v", gotOrder, wantOrder)
	}
}

func TestDetermineReturnsCannotDetermineForEncryptedIoStore(t *testing.T) {
	// Arrange
	fake := &fakeCaller{responses: map[string]string{
		"is_iostore_encrypted": `{"encrypted":true}`,
	}}
	entry := iostoreEntry("Mods/Example.utoc")

	// Act
	_, err := Determine(fake, "C:/root", entry)

	// Assert
	if !errors.Is(err, ErrCannotDetermineType) {
		t.Fatalf("Determine() error = %v, want ErrCannotDetermineType", err)
	}
	for _, action := range fake.actions() {
		if action == "list_iostore_files" {
			t.Errorf("Determine() called list_iostore_files for an encrypted container, want it skipped")
		}
	}
}

func TestDetermineReturnsCannotDetermineForMissingUTOCSidecar(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}
	entry := iostoreEntry("")

	// Act
	_, err := Determine(fake, "C:/root", entry)

	// Assert
	if !errors.Is(err, ErrCannotDetermineType) {
		t.Fatalf("Determine() error = %v, want ErrCannotDetermineType", err)
	}
	if len(fake.calls) != 0 {
		t.Errorf("Determine() made %d calls, want 0", len(fake.calls))
	}
}

func TestDetermineReturnsCannotDetermineForUnrecognizedBundleFormat(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}
	entry := discovery.Entry{ID: "none-1", Kind: discovery.EntryMod, BundleFormat: discovery.BundleFormatNone}

	// Act
	_, err := Determine(fake, "C:/root", entry)

	// Assert
	if !errors.Is(err, ErrCannotDetermineType) {
		t.Fatalf("Determine() error = %v, want ErrCannotDetermineType", err)
	}
}

func TestDeterminePropagatesUnderlyingCallError(t *testing.T) {
	// Arrange
	fake := &fakeCaller{errs: map[string]error{
		"list_pak": &uassettool.ToolError{Action: "list_pak", Message: "PAK file not found"},
	}}
	entry := classicEntry("Mods/Example.pak")

	// Act
	_, err := Determine(fake, "C:/root", entry)

	// Assert
	var toolErr *uassettool.ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("Determine() error = %v, want *uassettool.ToolError", err)
	}
}

const pinnedWorkerSourceRevision = "952bd331976c6f28efb36ca320c82c27e2456023"

func pinnedWorkerExecutablePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "build", "uassettool", "UAssetTool.exe"))
	if err != nil {
		t.Fatalf("resolve pinned worker path: %v", err)
	}
	return path
}

// Builds a real classic PAK fixture through the pinned worker's own writer,
// naming its one entry so Classify recognizes it as a skeletal mesh.
func buildClassicMeshFixture(t *testing.T, worker *uassettool.Worker, root string) string {
	t.Helper()

	contentPath := filepath.Join(root, "SK_Fixture.uasset")
	if err := os.WriteFile(contentPath, []byte("fixture content"), 0o600); err != nil {
		t.Fatalf("write pak fixture content: %v", err)
	}

	relativePakPath := "fixture.pak"
	params := map[string]any{
		"output_path": filepath.Join(root, relativePakPath),
		"file_paths":  []string{contentPath},
	}
	if err := worker.Call("create_pak", params, nil); err != nil {
		t.Fatalf("create_pak fixture: %v", err)
	}
	return relativePakPath
}

// Builds a real IoStore fixture the same way. Its container legitimately
// has zero Zen packages (no real .uasset input), so Determine is expected
// to succeed with CategoryUnknown here — this test verifies the real
// supervised-worker plumbing, not fixture content, matching
// uassettool.TestOperationsAgainstSupervisedWorkerAndFixtureArchives's own
// documented limitation for the same reason.
func buildIoStoreFixture(t *testing.T, worker *uassettool.Worker, root string) string {
	t.Helper()

	inputDir := filepath.Join(root, "iostore-input")
	if err := os.MkdirAll(inputDir, 0o700); err != nil {
		t.Fatalf("create iostore fixture input dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "readme.txt"), []byte("fixture content"), 0o600); err != nil {
		t.Fatalf("write iostore fixture content: %v", err)
	}

	relativeUTOCPath := "fixture.utoc"
	params := map[string]any{
		"output_path": filepath.Join(root, "fixture"),
		"input_dir":   inputDir,
		"hybrid":      true,
	}
	if err := worker.Call("create_mod_iostore", params, nil); err != nil {
		t.Fatalf("create_mod_iostore fixture: %v", err)
	}
	return relativeUTOCPath
}

// Exercises Determine's full path against a real supervised worker and
// disposable fixture archives. Skips if the pinned worker has not been
// fetched, since check.ps1 and normal development must not require network
// access; run fetch-uassettool.ps1 first to enable this test locally.
func TestDetermineAgainstSupervisedWorkerAndFixtureArchives(t *testing.T) {
	// Arrange
	executablePath := pinnedWorkerExecutablePath(t)
	if _, err := os.Stat(executablePath); err != nil {
		t.Skipf("pinned worker not found at %s; run fetch-uassettool.ps1 first (see docs/decisions/0004-pin-uassettool-worker.md)", executablePath)
	}

	worker, err := uassettool.NewWorker(uassettool.WorkerConfig{
		ExecutablePath:         executablePath,
		ExpectedSourceRevision: pinnedWorkerSourceRevision,
		CallTimeout:            30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	root := t.TempDir()

	t.Run("classic pak", func(t *testing.T) {
		// Arrange
		pakPath := buildClassicMeshFixture(t, worker, root)
		entry := classicEntry(pakPath)

		// Act
		category, err := Determine(worker, root, entry)

		// Assert
		if err != nil {
			t.Fatalf("Determine() error = %v", err)
		}
		if category != CategoryMesh {
			t.Errorf("Determine() = %q, want %q", category, CategoryMesh)
		}
	})

	t.Run("iostore", func(t *testing.T) {
		// Arrange
		utocPath := buildIoStoreFixture(t, worker, root)
		entry := iostoreEntry(utocPath)

		// Act
		category, err := Determine(worker, root, entry)

		// Assert
		if err != nil {
			t.Fatalf("Determine() error = %v", err)
		}
		if category != CategoryUnknown {
			t.Errorf("Determine() = %q, want %q (fixture has no real Zen package content)", category, CategoryUnknown)
		}
	})
}
