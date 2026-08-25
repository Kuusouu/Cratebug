package uassettool

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Records the last call it received and answers with either a canned error
// or a caller-supplied response body, without needing a live worker process.
type fakeCaller struct {
	action  string
	params  map[string]any
	respond func(result any) error
	err     error
}

func (f *fakeCaller) Call(action string, params map[string]any, result any) error {
	f.action = action
	f.params = params
	if f.err != nil {
		return f.err
	}
	if f.respond != nil {
		return f.respond(result)
	}
	return nil
}

func respondWithJSON(body string) func(result any) error {
	return func(result any) error {
		return json.Unmarshal([]byte(body), result)
	}
}

func TestListPakRejectsEmptyPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	_, err := ListPak(fake, "")

	// Assert
	if err == nil {
		t.Fatal("ListPak() error = nil, want an error for an empty path")
	}
	if fake.action != "" {
		t.Errorf("ListPak() called the worker with an empty path, want no call")
	}
}

func TestListPakDecodesEntries(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":[{"path":"/Game/Foo.uasset","size":10,"compressed_size":5,"encrypted":false,"compressed":true}]}`)}

	// Act
	entries, err := ListPak(fake, "mod.pak")

	// Assert
	if err != nil {
		t.Fatalf("ListPak() error = %v, want nil", err)
	}
	if fake.action != "list_pak" {
		t.Errorf("action = %q, want list_pak", fake.action)
	}
	if fake.params["file_path"] != "mod.pak" {
		t.Errorf("params[file_path] = %v, want mod.pak", fake.params["file_path"])
	}
	want := []PakEntry{{Path: "/Game/Foo.uasset", Size: 10, CompressedSize: 5, Compressed: true}}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %#v, want %#v", entries, want)
	}
}

// A file entry the worker reports with an empty path cannot be a real asset
// path, so it must be rejected here rather than handed to Cratebug's domain layer.
func TestListPakRejectsEntryWithEmptyPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":[{"path":"","size":0}]}`)}

	// Act
	_, err := ListPak(fake, "mod.pak")

	// Assert
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("ListPak() error = %v, want ErrMalformedResponse", err)
	}
}

func TestListPakPropagatesCallError(t *testing.T) {
	// Arrange
	fake := &fakeCaller{err: &ToolError{Action: "list_pak", Message: "PAK file not found: mod.pak"}}

	// Act
	_, err := ListPak(fake, "mod.pak")

	// Assert
	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("ListPak() error = %v, want *ToolError", err)
	}
}

func TestIsIoStoreEncryptedRejectsEmptyPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	_, err := IsIoStoreEncrypted(fake, "")

	// Assert
	if err == nil {
		t.Fatal("IsIoStoreEncrypted() error = nil, want an error for an empty path")
	}
	if fake.action != "" {
		t.Errorf("IsIoStoreEncrypted() called the worker with an empty path, want no call")
	}
}

func TestIsIoStoreEncryptedDecodesResult(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"encrypted":true}`)}

	// Act
	encrypted, err := IsIoStoreEncrypted(fake, "mod.utoc")

	// Assert
	if err != nil {
		t.Fatalf("IsIoStoreEncrypted() error = %v, want nil", err)
	}
	if !encrypted {
		t.Errorf("encrypted = false, want true")
	}
	if fake.action != "is_iostore_encrypted" {
		t.Errorf("action = %q, want is_iostore_encrypted", fake.action)
	}
	if fake.params["file_path"] != "mod.utoc" {
		t.Errorf("params[file_path] = %v, want mod.utoc", fake.params["file_path"])
	}
}

func TestListIoStoreFilesRejectsEmptyPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	_, err := ListIoStoreFiles(fake, "", "")

	// Assert
	if err == nil {
		t.Fatal("ListIoStoreFiles() error = nil, want an error for an empty path")
	}
	if fake.action != "" {
		t.Errorf("ListIoStoreFiles() called the worker with an empty path, want no call")
	}
}

func TestListIoStoreFilesDecodesFiles(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"package_count":1,"container_name":"fixture","files":["/Game/Foo"]}`)}

	// Act
	files, err := ListIoStoreFiles(fake, "mod.utoc", "")

	// Assert
	if err != nil {
		t.Fatalf("ListIoStoreFiles() error = %v, want nil", err)
	}
	if fake.action != "list_iostore_files" {
		t.Errorf("action = %q, want list_iostore_files", fake.action)
	}
	if fake.params["file_path"] != "mod.utoc" {
		t.Errorf("params[file_path] = %v, want mod.utoc", fake.params["file_path"])
	}
	if !reflect.DeepEqual(files, []string{"/Game/Foo"}) {
		t.Errorf("files = %#v, want [/Game/Foo]", files)
	}
}

func TestListIoStoreFilesOmitsAesKeyWhenEmpty(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":[]}`)}

	// Act
	if _, err := ListIoStoreFiles(fake, "mod.utoc", ""); err != nil {
		t.Fatalf("ListIoStoreFiles() error = %v, want nil", err)
	}

	// Assert
	if _, ok := fake.params["aes_key"]; ok {
		t.Errorf("params[aes_key] present = %v, want absent when aesKey is empty", fake.params["aes_key"])
	}
}

func TestListIoStoreFilesIncludesAesKeyWhenSet(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":[]}`)}

	// Act
	if _, err := ListIoStoreFiles(fake, "mod.utoc", "deadbeef"); err != nil {
		t.Fatalf("ListIoStoreFiles() error = %v, want nil", err)
	}

	// Assert
	if fake.params["aes_key"] != "deadbeef" {
		t.Errorf("params[aes_key] = %v, want deadbeef", fake.params["aes_key"])
	}
}

// An empty-string entry in the file list cannot be a real asset path, so it
// must be rejected here rather than handed to Cratebug's domain layer.
func TestListIoStoreFilesRejectsEntryWithEmptyPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":["/Game/Foo",""]}`)}

	// Act
	_, err := ListIoStoreFiles(fake, "mod.utoc", "")

	// Assert
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("ListIoStoreFiles() error = %v, want ErrMalformedResponse", err)
	}
}

func TestListIoStoreFilesPropagatesCallError(t *testing.T) {
	// Arrange
	fake := &fakeCaller{err: &ToolError{Action: "list_iostore_files", Message: "UTOC file not found: mod.utoc"}}

	// Act
	_, err := ListIoStoreFiles(fake, "mod.utoc", "")

	// Assert
	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("ListIoStoreFiles() error = %v, want *ToolError", err)
	}
}

// Pinned in docs/decisions/0004-pin-uassettool-worker.md; update both together.
const pinnedWorkerSourceRevision = PinnedSourceRevision

// Resolves the pinned worker fetched by fetch-uassettool.ps1 into build/uassettool.
func pinnedWorkerExecutablePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "build", "uassettool", "UAssetTool.exe"))
	if err != nil {
		t.Fatalf("resolve pinned worker path: %v", err)
	}
	return path
}

// Builds a real, valid classic PAK archive by asking the pinned worker to
// create one, rather than hand-rolling the PAK binary format. This keeps the
// fixture disposable and synthetic (CODING_GUIDELINES.md) while still being
// something the worker's own PakReader accepts.
func buildClassicPakFixture(t *testing.T, worker *Worker, dir string) string {
	t.Helper()

	contentPath := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(contentPath, []byte("fixture content"), 0o600); err != nil {
		t.Fatalf("write pak fixture content: %v", err)
	}

	pakPath := filepath.Join(dir, "fixture.pak")
	params := map[string]any{
		"output_path": pakPath,
		"file_paths":  []string{contentPath},
	}
	if err := worker.Call("create_pak", params, nil); err != nil {
		t.Fatalf("create_pak fixture: %v", err)
	}
	return pakPath
}

// Builds a real, valid IoStore container the same way: through the pinned
// worker's own writer. hybrid:true lets a plain non-Unreal file produce a
// valid container without needing a real serialized .uasset as input.
func buildIoStoreFixture(t *testing.T, worker *Worker, dir string) string {
	t.Helper()

	inputDir := filepath.Join(dir, "iostore-input")
	if err := os.MkdirAll(inputDir, 0o700); err != nil {
		t.Fatalf("create iostore fixture input dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "readme.txt"), []byte("fixture content"), 0o600); err != nil {
		t.Fatalf("write iostore fixture content: %v", err)
	}

	outputBase := filepath.Join(dir, "fixture")
	params := map[string]any{
		"output_path": outputBase,
		"input_dir":   inputDir,
		"hybrid":      true,
	}
	if err := worker.Call("create_mod_iostore", params, nil); err != nil {
		t.Fatalf("create_mod_iostore fixture: %v", err)
	}
	return outputBase + ".utoc"
}

// Exercises the full path this task's Verify clause asks for: Go caller to
// adapter to supervised worker and back, against real classic and IoStore
// archives. Skips if the pinned worker has not been fetched, since
// check.ps1 and normal development must not require network access; run
// fetch-uassettool.ps1 first to enable this test locally.
func TestOperationsAgainstSupervisedWorkerAndFixtureArchives(t *testing.T) {
	// Arrange
	executablePath := pinnedWorkerExecutablePath(t)
	if _, err := os.Stat(executablePath); err != nil {
		t.Skipf("pinned worker not found at %s; run fetch-uassettool.ps1 first (see docs/decisions/0004-pin-uassettool-worker.md)", executablePath)
	}

	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         executablePath,
		ExpectedSourceRevision: pinnedWorkerSourceRevision,
		CallTimeout:            30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	dir := t.TempDir()

	t.Run("classic pak", func(t *testing.T) {
		// Arrange
		pakPath := buildClassicPakFixture(t, worker, dir)

		// Act
		entries, err := ListPak(worker, pakPath)

		// Assert
		if err != nil {
			t.Fatalf("ListPak() error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("ListPak() returned %d entries, want 1: %#v", len(entries), entries)
		}
		if !strings.HasSuffix(entries[0].Path, "readme.txt") {
			t.Errorf("entries[0].Path = %q, want a path ending in readme.txt", entries[0].Path)
		}
	})

	t.Run("iostore", func(t *testing.T) {
		// Arrange
		utocPath := buildIoStoreFixture(t, worker, dir)

		// Act
		encrypted, err := IsIoStoreEncrypted(worker, utocPath)

		// Assert
		if err != nil {
			t.Fatalf("IsIoStoreEncrypted() error = %v", err)
		}
		if encrypted {
			t.Errorf("IsIoStoreEncrypted() = true, want false: fixture was built without obfuscation")
		}
	})

	t.Run("iostore file listing", func(t *testing.T) {
		// Arrange
		utocPath := buildIoStoreFixture(t, worker, dir)

		// Act
		files, err := ListIoStoreFiles(worker, utocPath, "")

		// Assert
		//
		// The hybrid fixture built by buildIoStoreFixture has no real
		// .uasset input, so its IoStore container legitimately has zero Zen
		// packages: the fixture's loose file lands in the companion PAK
		// (list_pak's job), not the .utoc/.ucas (list_iostore_files's job).
		// This still exercises the full Go-to-worker-and-back path for a
		// real, valid IoStore container; it just cannot assert non-empty
		// content without a genuine serialized .uasset fixture.
		if err != nil {
			t.Fatalf("ListIoStoreFiles() error = %v", err)
		}
		if files == nil {
			t.Errorf("ListIoStoreFiles() = nil, want a non-nil (possibly empty) slice")
		}
	})
}
