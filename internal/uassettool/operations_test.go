package uassettool

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestListPakWithKeyIncludesAesKeyWhenSet(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"files":[]}`)}

	// Act
	if _, err := ListPakWithKey(fake, "mod.pak", MarvelRivalsAESKey); err != nil {
		t.Fatalf("ListPakWithKey() error = %v, want nil", err)
	}

	// Assert
	if fake.params["aes_key"] != MarvelRivalsAESKey {
		t.Errorf("params[aes_key] = %v, want MarvelRivalsAESKey", fake.params["aes_key"])
	}
}

func TestExtractIoStoreRejectsEmptyPaths(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	_, err := ExtractIoStore(fake, "", "out", "")

	// Assert
	if err == nil {
		t.Fatal("ExtractIoStore() error = nil, want an error for an empty utoc path")
	}
	if fake.action != "" {
		t.Errorf("ExtractIoStore() called the worker, want no call")
	}
}

func TestExtractIoStoreDecodesCount(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"count":3}`)}

	// Act
	count, err := ExtractIoStore(fake, "mod.utoc", "out", MarvelRivalsAESKey)

	// Assert
	if err != nil {
		t.Fatalf("ExtractIoStore() error = %v, want nil", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
	if fake.action != "extract_iostore" {
		t.Errorf("action = %q, want extract_iostore", fake.action)
	}
	if fake.params["aes_key"] != MarvelRivalsAESKey {
		t.Errorf("params[aes_key] = %v, want MarvelRivalsAESKey", fake.params["aes_key"])
	}
}

func TestExtractPakAllDecodesCount(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"extracted_count":2}`)}

	// Act
	count, err := ExtractPakAll(fake, "mod.pak", "out", "")

	// Assert
	if err != nil {
		t.Fatalf("ExtractPakAll() error = %v, want nil", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if fake.action != "extract_pak_all" {
		t.Errorf("action = %q, want extract_pak_all", fake.action)
	}
}

func TestCreateModIoStoreSendsObfuscate(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"utoc_path":"out.utoc","ucas_path":"out.ucas","pak_path":"out.pak","converted_count":1,"file_count":2}`)}

	// Act
	result, err := CreateModIoStore(fake, "out", "in", IoStoreCreateOptions{Obfuscate: true, Hybrid: true})

	// Assert
	if err != nil {
		t.Fatalf("CreateModIoStore() error = %v, want nil", err)
	}
	if fake.action != "create_mod_iostore" {
		t.Errorf("action = %q, want create_mod_iostore", fake.action)
	}
	if fake.params["obfuscate"] != true {
		t.Errorf("params[obfuscate] = %v, want true", fake.params["obfuscate"])
	}
	if fake.params["hybrid"] != true {
		t.Errorf("params[hybrid] = %v, want true", fake.params["hybrid"])
	}
	if result.UTOCPath != "out.utoc" || result.FileCount != 2 {
		t.Errorf("result = %+v, want utoc out.utoc and file_count 2", result)
	}
}

func TestCreateModIoStoreSendsInputPak(t *testing.T) {
	// Arrange
	fake := &fakeCaller{respond: respondWithJSON(`{"utoc_path":"out.utoc","ucas_path":"out.ucas","pak_path":"out.pak","converted_count":0,"file_count":1}`)}

	// Act
	_, err := CreateModIoStore(fake, "out", "", IoStoreCreateOptions{Hybrid: true, InputPak: "in.pak", Obfuscate: true})

	// Assert
	if err != nil {
		t.Fatalf("CreateModIoStore() error = %v, want nil", err)
	}
	if fake.params["input_pak"] != "in.pak" {
		t.Errorf("params[input_pak] = %v, want in.pak", fake.params["input_pak"])
	}
	if _, ok := fake.params["input_dir"]; ok {
		t.Errorf("params[input_dir] = %v, want omitted", fake.params["input_dir"])
	}
}

func TestCompanionPakHasRawFilesIgnoresMetadata(t *testing.T) {
	// Arrange
	entries := []PakEntry{{Path: "chunknames"}, {Path: "patched_files"}}

	// Act / Assert
	if CompanionPakHasRawFiles(entries) {
		t.Fatal("CompanionPakHasRawFiles() = true, want false for metadata-only listings")
	}
	if !CompanionPakHasRawFiles([]PakEntry{{Path: "Audio/sound.bnk"}}) {
		t.Fatal("CompanionPakHasRawFiles() = false, want true for a raw file")
	}
}

func TestCompanionPakHasRawFilesIgnoresCleanupStub(t *testing.T) {
	// Arrange
	entries := []PakEntry{
		{Path: CompanionStubName},
		{Path: "../../../" + CompanionStubName},
	}

	// Act / Assert
	if CompanionPakHasRawFiles(entries) {
		t.Fatal("CompanionPakHasRawFiles() = true, want false for the cleanup stub")
	}
	if !IsCompanionStubPath("Content/" + CompanionStubName) {
		t.Fatal("IsCompanionStubPath() = false, want true")
	}
	if IsCompanionStubPath("Audio/sound.bnk") {
		t.Fatal("IsCompanionStubPath() = true for a raw file")
	}
}

func TestIsCompanionMetadataPathMatchesMountPrefixes(t *testing.T) {
	// Arrange / Act / Assert
	for _, path := range []string{
		"chunknames",
		"../../..//chunknames",
		"../../../patched_files",
		"Content/patched_files",
	} {
		if !IsCompanionMetadataPath(path) {
			t.Errorf("IsCompanionMetadataPath(%q) = false, want true", path)
		}
	}
	if IsCompanionMetadataPath("Audio/sound.bnk") {
		t.Fatal("IsCompanionMetadataPath() = true for a raw file")
	}
}

func TestCompanionPakUnsupportedPathsReturnsMatches(t *testing.T) {
	// Arrange
	entries := []PakEntry{
		{Path: "../../../patched_files"},
		{Path: "Audio/sound.bnk"},
		{Path: "../../..//chunknames"},
	}

	// Act
	got := CompanionPakUnsupportedPaths(entries)

	// Assert
	if len(got) != 2 || got[0] != "../../../patched_files" || got[1] != "../../..//chunknames" {
		t.Fatalf("CompanionPakUnsupportedPaths() = %v, want the two metadata paths", got)
	}
}

func TestCreatePakRejectsEmptyOutput(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	err := CreatePak(fake, "", nil, "")

	// Assert
	if err == nil {
		t.Fatal("CreatePak() error = nil, want an error for an empty output path")
	}
	if fake.action != "" {
		t.Error("CreatePak() called the worker with an empty path, want no call")
	}
}

func TestCreatePakRejectsEmptyFileList(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	err := CreatePak(fake, "out.pak", nil, "../../../")

	// Assert
	if err == nil {
		t.Fatal("CreatePak() error = nil, want an error for an empty file list")
	}
	if fake.action != "" {
		t.Error("CreatePak() called the worker with no files, want no call")
	}
}

func TestCreatePakSendsMappedPaths(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}
	files := []string{".cratebug-companion-stub=C:\\tmp\\stub"}

	// Act
	if err := CreatePak(fake, "out.pak", files, "../../../"); err != nil {
		t.Fatalf("CreatePak() error = %v, want nil", err)
	}

	// Assert
	if fake.action != "create_pak" {
		t.Errorf("action = %q, want create_pak", fake.action)
	}
	if fake.params["mount_point"] != "../../../" {
		t.Errorf("params[mount_point] = %v, want ../../../", fake.params["mount_point"])
	}
	paths, ok := fake.params["file_paths"].([]string)
	if !ok || len(paths) != 1 || paths[0] != files[0] {
		t.Errorf("params[file_paths] = %v, want %v", fake.params["file_paths"], files)
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
		CallTimeout:            DefaultWriteCallTimeout,
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

	t.Run("iostore obfuscate round trip", func(t *testing.T) {
		// Arrange
		inputDir := filepath.Join(dir, "obfuscate-input")
		if err := os.MkdirAll(inputDir, 0o700); err != nil {
			t.Fatalf("create obfuscate input dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(inputDir, "readme.txt"), []byte("obfuscate fixture"), 0o600); err != nil {
			t.Fatalf("write obfuscate fixture content: %v", err)
		}
		plainBase := filepath.Join(dir, "plain")
		if _, err := CreateModIoStore(worker, plainBase, inputDir, IoStoreCreateOptions{Hybrid: true}); err != nil {
			t.Fatalf("CreateModIoStore() plain: %v", err)
		}
		plainUTOC := plainBase + ".utoc"
		if encrypted, err := IsIoStoreEncrypted(worker, plainUTOC); err != nil || encrypted {
			t.Fatalf("plain IsIoStoreEncrypted() = %v, %v, want false", encrypted, err)
		}

		extractDir := filepath.Join(dir, "extracted")
		if err := os.MkdirAll(extractDir, 0o700); err != nil {
			t.Fatalf("create extract dir: %v", err)
		}
		if _, err := ExtractIoStore(worker, plainUTOC, extractDir, ""); err != nil {
			t.Fatalf("ExtractIoStore() plain: %v", err)
		}
		// Hybrid-only fixtures keep loose files in the companion PAK at
		// leading-slash paths. extract_pak_all drops those on Windows
		// (extracted_count=0). input_pak is the worker's extract that trims
		// slashes, so this is still extract → create.
		encryptedBase := filepath.Join(dir, "encrypted")
		if _, err := CreateModIoStore(worker, encryptedBase, "", IoStoreCreateOptions{
			Obfuscate: true,
			Hybrid:    true,
			InputPak:  plainBase + ".pak",
		}); err != nil {
			t.Fatalf("CreateModIoStore() obfuscate: %v (pinned worker may lack obfuscate; re-pin per 0004)", err)
		}
		encryptedUTOC := encryptedBase + ".utoc"
		encryptedFlag, err := IsIoStoreEncrypted(worker, encryptedUTOC)
		if err != nil {
			t.Fatalf("IsIoStoreEncrypted() encrypted: %v", err)
		}
		if !encryptedFlag {
			t.Fatal("IsIoStoreEncrypted() = false after obfuscate, want true")
		}

		// Act
		decryptDir := filepath.Join(dir, "decrypted-extract")
		if err := os.MkdirAll(decryptDir, 0o700); err != nil {
			t.Fatalf("create decrypt extract dir: %v", err)
		}
		if _, err := ExtractIoStore(worker, encryptedUTOC, decryptDir, MarvelRivalsAESKey); err != nil {
			t.Fatalf("ExtractIoStore() encrypted: %v", err)
		}
		decryptedBase := filepath.Join(dir, "decrypted")
		if _, err := CreateModIoStore(worker, decryptedBase, "", IoStoreCreateOptions{
			Hybrid:   true,
			InputPak: encryptedBase + ".pak",
			AESKey:   MarvelRivalsAESKey,
		}); err != nil {
			t.Fatalf("CreateModIoStore() decrypt: %v", err)
		}

		// Assert
		decryptedFlag, err := IsIoStoreEncrypted(worker, decryptedBase+".utoc")
		if err != nil {
			t.Fatalf("IsIoStoreEncrypted() decrypted: %v", err)
		}
		if decryptedFlag {
			t.Fatal("IsIoStoreEncrypted() = true after decrypt rebuild, want false")
		}
		// This fixture is hybrid-only (readme.txt, no meshes). extract→create
		// did not need a .usmap. Real mesh mods may still need one.
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

	t.Run("create_pak stub drops companion metadata names", func(t *testing.T) {
		// Arrange
		inputDir := filepath.Join(dir, "companion-input")
		if err := os.MkdirAll(inputDir, 0o700); err != nil {
			t.Fatalf("create companion input dir: %v", err)
		}
		chunknames := filepath.Join(inputDir, "chunknames")
		patched := filepath.Join(inputDir, "patched_files")
		raw := filepath.Join(inputDir, "readme.txt")
		for path, body := range map[string]string{
			chunknames: "names",
			patched:    "patched",
			raw:        "keep",
		} {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
		dirty := filepath.Join(dir, "dirty-companion.pak")
		if err := CreatePak(worker, dirty, []string{chunknames, patched, raw}, "../../../"); err != nil {
			t.Fatalf("CreatePak() dirty: %v", err)
		}
		listed, err := ListPak(worker, dirty)
		if err != nil {
			t.Fatalf("ListPak() dirty: %v", err)
		}
		if len(CompanionPakUnsupportedPaths(listed)) == 0 {
			t.Fatalf("dirty listing had no metadata names: %#v", listed)
		}

		// Act
		// The worker rejects an empty file list. A metadata-only companion
		// is rewritten with one harmless placeholder instead.
		stubFile := filepath.Join(inputDir, ".cratebug-companion-stub")
		if err := os.WriteFile(stubFile, []byte("cratebug companion stub\n"), 0o600); err != nil {
			t.Fatalf("write stub file: %v", err)
		}
		stub := filepath.Join(dir, "stub-companion.pak")
		if err := CreatePak(worker, stub, []string{".cratebug-companion-stub=" + stubFile}, "../../../"); err != nil {
			t.Fatalf("CreatePak() stub: %v", err)
		}
		clean, err := ListPak(worker, stub)

		// Assert
		if err != nil {
			t.Fatalf("ListPak() stub: %v", err)
		}
		if got := CompanionPakUnsupportedPaths(clean); len(got) != 0 {
			t.Fatalf("stub listing still has metadata names: %v", got)
		}
	})
}
