package mutation

import (
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

type scriptedEncryptionCaller struct {
	encryptedByUTOC map[string]bool
	failPakFixer    bool
	failListPak     bool
	pakListing      string
	createPakBody   string
	pakFixerTargets []string
}

func (s *scriptedEncryptionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "is_iostore_encrypted":
		path, _ := params["file_path"].(string)
		// Staged decryption changes the real TOC flag; the encrypt mock uses a marker.
		if filepath.Base(path) == "bundle.utoc" {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			encrypted := string(body) == "direct.utoc"
			if len(body) > 80 {
				encrypted = body[80]&2 != 0
			}
			return json.Unmarshal([]byte(encryptionJSON(encrypted)), result)
		}
		return json.Unmarshal([]byte(encryptionJSON(s.encryptedByUTOC[filepath.Base(path)])), result)
	case "pak_fixer":
		if s.failPakFixer {
			return &uassettool.ToolError{Action: action, Message: "encrypt failed"}
		}
		path, _ := params["file_path"].(string)
		s.pakFixerTargets = append(s.pakFixerTargets, filepath.Base(path))
		if params["obfuscate"] != true {
			return &uassettool.ToolError{Action: action, Message: "obfuscate is required"}
		}
		// Mark the staged copies so the test can tell replacement ran.
		base := path[:len(path)-len(filepath.Ext(path))]
		for _, ext := range []string{".pak", ".utoc", ".ucas"} {
			if err := os.WriteFile(base+ext, []byte("direct"+ext), 0o600); err != nil {
				return err
			}
		}
		return json.Unmarshal([]byte(`{"total":1,"fixed":0,"already_clean":1,"failed":0,"containers_encrypted":1,"containers_already_encrypted":0,"containers_failed":0}`), result)
	case "list_pak":
		if s.failListPak {
			return &uassettool.ToolError{Action: action, Message: "list failed"}
		}
		listing := s.pakListing
		if listing == "" {
			listing = `{"files":[]}`
		}
		return json.Unmarshal([]byte(listing), result)
	case "extract_pak_all":
		return json.Unmarshal([]byte(`{"extracted_count":0}`), result)
	case "create_pak":
		output, _ := params["output_path"].(string)
		body := s.createPakBody
		if body == "" {
			body = "stripped.pak"
		}
		return os.WriteFile(output, []byte(body), 0o600)
	default:
		return &uassettool.ToolError{Action: action, Message: "unexpected archive operation"}
	}
}

func encryptionJSON(encrypted bool) string {
	if encrypted {
		return `{"encrypted":true}`
	}
	return `{"encrypted":false}`
}

func writeIoStoreBundle(t *testing.T, root, folder, stem string, disabled bool) {
	t.Helper()
	dir := filepath.Join(root, folder)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}
	primary := stem + ".pak"
	if disabled {
		primary = stem + ".pak_crateoff"
	}
	for _, name := range []string{primary, stem + ".utoc", stem + ".ucas"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("live-"+name), 0o600); err != nil {
			t.Fatalf("write bundle file: %v", err)
		}
	}
}

func TestSetModEncryptionRejectsClassicPak(t *testing.T) {
	// Arrange
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Classic.pak"), []byte("pak"), 0o600); err != nil {
		t.Fatalf("write classic pak: %v", err)
	}
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	var classicID string
	for _, entry := range library.Entries {
		if entry.BundleFormat == discovery.BundleFormatClassic {
			classicID = entry.ID
		}
	}
	if classicID == "" {
		t.Fatal("Scan() found no classic entry")
	}

	// Act
	_, err = SetModEncryption(root, []string{classicID}, true, &scriptedEncryptionCaller{}, nil, nil)

	// Assert
	if !errors.Is(err, ErrEncryptionIneligible) {
		t.Fatalf("SetModEncryption() error = %v, want ErrEncryptionIneligible", err)
	}
}

func TestSetModEncryptionRejectsMixedState(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Alpha_9999999_P", false)
	writeIoStoreBundle(t, root, "", "Beta_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(library.Entries) != 2 {
		t.Fatalf("Scan() entries = %d, want 2", len(library.Entries))
	}
	caller := &scriptedEncryptionCaller{encryptedByUTOC: map[string]bool{
		"Alpha_9999999_P.utoc": true,
		"Beta_9999999_P.utoc":  false,
	}}

	// Act
	_, err = SetModEncryption(root, []string{library.Entries[0].ID, library.Entries[1].ID}, true, caller, nil, nil)

	// Assert
	if !errors.Is(err, ErrEncryptionMixedState) {
		t.Fatalf("SetModEncryption() error = %v, want ErrEncryptionMixedState", err)
	}
}

func TestSetModEncryptionPreservesDisabledPrimary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "skins", "Hero_9999999_P", true)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(library.Entries) != 1 {
		t.Fatalf("Scan() entries = %d, want 1", len(library.Entries))
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{encryptedByUTOC: map[string]bool{
		"Hero_9999999_P.utoc": false,
	}}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, true, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 || result.Succeeded[0] != entry.ID {
		t.Fatalf("Succeeded = %v, want [%s]", result.Succeeded, entry.ID)
	}
	primary := filepath.Join(root, "skins", "Hero_9999999_P.pak_crateoff")
	if _, err := os.Stat(primary); err != nil {
		t.Fatalf("disabled primary missing after encrypt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skins", "Hero_9999999_P.pak")); !os.IsNotExist(err) {
		t.Fatal("encrypt wrote an enabled .pak next to a disabled primary")
	}
	body, err := os.ReadFile(primary)
	if err != nil {
		t.Fatalf("read rebuilt primary: %v", err)
	}
	if string(body) != "direct.pak" {
		t.Errorf("primary content = %q, want direct.pak", body)
	}
}

func TestSetModEncryptionRollsBackFailedRebuild(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": false},
		failPakFixer:    true,
	}
	original, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, true, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil with a per-mod failure", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want one failure", result.Failed)
	}
	after, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read after failed encrypt: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("primary changed after failed rebuild: %q", after)
	}
}

func TestSetModEncryptionFailsClosedWhenCompanionPakListFails(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": true},
		failListPak:     true,
	}
	original, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, false, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil with a per-mod failure", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want one failure", result.Failed)
	}
	after, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read after failed encrypt: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("primary changed after companion PAK list failure: %q", after)
	}
}

func TestSetModEncryptionSkipsWhenAlreadyAtTarget(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{encryptedByUTOC: map[string]bool{
		"Hero_9999999_P.utoc": true,
	}}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, true, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 {
		t.Fatalf("Succeeded = %v, want the already-encrypted id", result.Succeeded)
	}
	body, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read primary: %v", err)
	}
	if string(body) != "live-Hero_9999999_P.pak" {
		t.Errorf("already-encrypted bundle was rewritten: %q", body)
	}
}

func TestSetModEncryptionDecryptsWithoutChangingIdentity(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	plainTOC, plainUCAS := writeEncryptedIoStore(t, root, "Hero_9999999_P")
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": true},
		pakListing:      metadataListing(),
		createPakBody:   "stripped-companion",
	}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, false, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %v, want none", result.Failed)
	}
	body, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read primary: %v", err)
	}
	if string(body) != "stripped-companion" {
		t.Errorf("primary content = %q, want the stripped companion PAK", body)
	}
	for extension, want := range map[string][]byte{".utoc": plainTOC, ".ucas": plainUCAS} {
		got, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P"+extension))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("decrypted %s differs from the original clear bytes", extension)
		}
	}
}

func TestSetModEncryptionIgnoresCompanionCleanupStub(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": false},
		pakListing:      `{"files":[{"path":"` + uassettool.CompanionStubName + `"}]}`,
	}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, true, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %v, want none for a stub-only companion PAK", result.Failed)
	}
	if len(result.Succeeded) != 1 {
		t.Fatalf("Succeeded = %v, want [%s]", result.Succeeded, entry.ID)
	}
}

func TestSetModEncryptionPooledRewritesEachTarget(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Alpha_9999999_P", false)
	writeIoStoreBundle(t, root, "", "Beta_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(library.Entries) != 2 {
		t.Fatalf("Scan() entries = %d, want 2", len(library.Entries))
	}
	ids := []string{library.Entries[0].ID, library.Entries[1].ID}
	var launched atomic.Int32
	launch := func() (ArchiveCaller, func(), error) {
		launched.Add(1)
		return &scriptedEncryptionCaller{encryptedByUTOC: map[string]bool{
			"Alpha_9999999_P.utoc": false,
			"Beta_9999999_P.utoc":  false,
		}}, func() {}, nil
	}

	// Act
	result, err := SetModEncryptionPooled(root, ids, true, launch, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryptionPooled() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 2 {
		t.Fatalf("Succeeded = %v, want both ids", result.Succeeded)
	}
	wantWorkers := uassettool.DefaultWorkerPoolSizeForLibrary(2)
	if wantWorkers > 2 {
		wantWorkers = 2
	}
	if got := int(launched.Load()); got != wantWorkers {
		t.Errorf("launched workers = %d, want %d", got, wantWorkers)
	}
	for _, name := range []string{"Alpha_9999999_P.pak", "Beta_9999999_P.pak"} {
		body, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		if string(body) != "direct.pak" {
			t.Errorf("%s content = %q, want direct.pak", name, body)
		}
	}
}

func TestFindModsNeedingEncryption(t *testing.T) {
	ui := encryptableIoStoreEntry("ui-mod")
	mesh := encryptableIoStoreEntry("mesh-mod")
	already := encryptableIoStoreEntry("encrypted-ui")
	classic := discovery.Entry{
		ID:           "classic-ui",
		Kind:         discovery.EntryMod,
		PrimaryPath:  "classic.pak",
		DisplayName:  "classic",
		BundleFormat: discovery.BundleFormatClassic,
	}
	entries := []discovery.Entry{ui, mesh, already, classic}
	identities := map[string]modtype.Identity{
		already.ID: {Encrypted: true},
	}
	paths := map[string][]string{
		ui.ID:      {"/Game/Marvel/UI/Icons/Icon.uasset"},
		mesh.ID:    {"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"},
		already.ID: {"/Game/Marvel/UI/Icons/Icon.uasset"},
		classic.ID: {"UI/Icons/Icon.uasset"},
		"missing":  {"UI/Icons/Icon.uasset"},
	}

	// Act
	got := FindModsNeedingEncryption(entries, identities, paths)

	// Assert
	if len(got) != 1 || got[0] != ui.ID {
		t.Fatalf("FindModsNeedingEncryption() = %v, want [%s]", got, ui.ID)
	}
}

func encryptableIoStoreEntry(id string) discovery.Entry {
	return discovery.Entry{
		ID:           id,
		Kind:         discovery.EntryMod,
		PrimaryPath:  id + ".pak",
		DisplayName:  id,
		BundleFormat: discovery.BundleFormatIoStore,
		Sidecars: discovery.Sidecars{
			UTOC: id + ".utoc",
			UCAS: id + ".ucas",
		},
	}
}

func TestSetModEncryptionRejectsInvalidTOCBeforeReplacement(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedEncryptionCaller{
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": true},
	}

	// Act
	result, err := SetModEncryption(root, []string{entry.ID}, false, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("SetModEncryption() error = %v, want nil with per-mod failure", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want one failure", result.Failed)
	}
	if !strings.Contains(result.Failed[0].Message, "invalid IoStore TOC header") {
		t.Errorf("failure message = %q, want an invalid TOC error", result.Failed[0].Message)
	}
	for _, extension := range []string{".pak", ".utoc", ".ucas"} {
		name := "Hero_9999999_P" + extension
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "live-"+name {
			t.Errorf("failed decrypt changed %s", name)
		}
	}
}

func writeEncryptedIoStore(t *testing.T, root, stem string) ([]byte, []byte) {
	t.Helper()
	// One uncompressed block keeps the replacement check independent of the worker mock.
	const headerSize = 144
	const chunkIDSize = 12
	const offsetLengthSize = 10
	const compressionEntrySize = 12
	toc := make([]byte, headerSize+chunkIDSize+offsetLengthSize+compressionEntrySize)
	copy(toc, "-==--==--==--==-")
	toc[16] = 5
	binary.LittleEndian.PutUint32(toc[20:24], headerSize)
	binary.LittleEndian.PutUint32(toc[24:28], 1)
	binary.LittleEndian.PutUint32(toc[28:32], 1)
	binary.LittleEndian.PutUint32(toc[32:36], compressionEntrySize)
	binary.LittleEndian.PutUint32(toc[44:48], 16)
	binary.LittleEndian.PutUint32(toc[52:56], 1)
	binary.LittleEndian.PutUint64(toc[56:64], 42)
	copy(toc[headerSize:headerSize+chunkIDSize], []byte("asset-id-123"))
	entry := toc[headerSize+chunkIDSize+offsetLengthSize:]
	entry[5], entry[8] = 16, 16
	plainUCAS := []byte("cooked asset 123")
	key, err := hex.DecodeString(uassettool.MarvelRivalsAESKey)
	if err != nil {
		t.Fatal(err)
	}
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encryptedUCAS := make([]byte, len(plainUCAS))
	blockCipher.Encrypt(encryptedUCAS, plainUCAS)
	plainTOC := append([]byte(nil), toc...)
	toc[80] = 2
	for extension, body := range map[string][]byte{".utoc": toc, ".ucas": encryptedUCAS} {
		if err := os.WriteFile(filepath.Join(root, stem+extension), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return plainTOC, plainUCAS
}
