package mutation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

type scriptedEncryptionCaller struct {
	encryptedByUTOC map[string]bool
	failCreate      bool
	failListPak     bool
}

func (s *scriptedEncryptionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "is_iostore_encrypted":
		path, _ := params["file_path"].(string)
		return json.Unmarshal([]byte(encryptionJSON(s.encryptedByUTOC[filepath.Base(path)])), result)
	case "extract_iostore":
		output, _ := params["output_path"].(string)
		return os.WriteFile(filepath.Join(output, "extracted.txt"), []byte("extracted"), 0o600)
	case "list_pak":
		if s.failListPak {
			return &uassettool.ToolError{Action: action, Message: "list failed"}
		}
		return json.Unmarshal([]byte(`{"files":[]}`), result)
	case "extract_pak_all":
		return json.Unmarshal([]byte(`{"extracted_count":0}`), result)
	case "create_mod_iostore":
		if s.failCreate {
			return &uassettool.ToolError{Action: action, Message: "rebuild failed"}
		}
		output, _ := params["output_path"].(string)
		for _, ext := range []string{".pak", ".utoc", ".ucas"} {
			if err := os.WriteFile(output+ext, []byte("rebuilt"+ext), 0o600); err != nil {
				return err
			}
		}
		payload := `{"utoc_path":"` + filepath.ToSlash(output) + `.utoc","ucas_path":"` + filepath.ToSlash(output) + `.ucas","pak_path":"` + filepath.ToSlash(output) + `.pak","converted_count":1,"file_count":1}`
		return json.Unmarshal([]byte(payload), result)
	default:
		return nil
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
	if string(body) != "rebuilt.pak" {
		t.Errorf("primary content = %q, want rebuilt.pak", body)
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
		failCreate:      true,
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
		encryptedByUTOC: map[string]bool{"Hero_9999999_P.utoc": false},
		failListPak:     true,
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
