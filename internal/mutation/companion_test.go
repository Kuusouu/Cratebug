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

type scriptedCompanionCaller struct {
	listingByPak    map[string]string
	failListByPak   map[string]bool
	failCreate      bool
	extractWrites   map[string]string
	createFileCount int
}

func (s *scriptedCompanionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "list_pak":
		path, _ := params["file_path"].(string)
		base := filepath.Base(path)
		if s.failListByPak[base] {
			return &uassettool.ToolError{Action: action, Message: "list failed"}
		}
		body := s.listingByPak[base]
		if body == "" {
			body = `{"files":[]}`
		}
		return json.Unmarshal([]byte(body), result)
	case "is_iostore_encrypted":
		return json.Unmarshal([]byte(`{"encrypted":false}`), result)
	case "extract_pak_all":
		output, _ := params["output_path"].(string)
		for name, body := range s.extractWrites {
			if err := os.WriteFile(filepath.Join(output, name), []byte(body), 0o600); err != nil {
				return err
			}
		}
		return json.Unmarshal([]byte(`{"extracted_count":1}`), result)
	case "create_pak":
		if s.failCreate {
			return &uassettool.ToolError{Action: action, Message: "rebuild failed"}
		}
		output, _ := params["output_path"].(string)
		s.createFileCount++
		return os.WriteFile(output, []byte("rewritten-companion"), 0o600)
	default:
		return nil
	}
}

func metadataListing() string {
	return `{"files":[{"path":"../../..//chunknames","size":1},{"path":"../../../patched_files","size":1}]}`
}

func hybridListing() string {
	return `{"files":[{"path":"../../..//chunknames","size":1},{"path":"Audio/sound.bnk","size":4}]}`
}

func TestFindUnsupportedCompanionPaksFindsDirtyAndSkipsListErrors(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Dirty_9999999_P", false)
	writeIoStoreBundle(t, root, "", "Broken_9999999_P", false)
	writeIoStoreBundle(t, root, "", "Clean_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	ids := map[string]string{}
	for _, entry := range library.Entries {
		ids[filepath.Base(entry.PrimaryPath)] = entry.ID
	}
	caller := &scriptedCompanionCaller{
		listingByPak: map[string]string{
			"Dirty_9999999_P.pak": metadataListing(),
			"Clean_9999999_P.pak": `{"files":[{"path":"Audio/sound.bnk","size":4}]}`,
		},
		failListByPak: map[string]bool{"Broken_9999999_P.pak": true},
	}

	// Act
	found, err := FindUnsupportedCompanionPaks(root, caller)

	// Assert
	if err != nil {
		t.Fatalf("FindUnsupportedCompanionPaks() error = %v, want nil", err)
	}
	if len(found) != 1 || found[0] != ids["Dirty_9999999_P.pak"] {
		t.Fatalf("FindUnsupportedCompanionPaks() = %v, want only the dirty id", found)
	}
}

func TestStripCompanionPaksRewritesMetadataOnlyPrimary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "skins", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedCompanionCaller{
		listingByPak: map[string]string{"Hero_9999999_P.pak": metadataListing()},
	}
	originalUTOC, err := os.ReadFile(filepath.Join(root, "skins", "Hero_9999999_P.utoc"))
	if err != nil {
		t.Fatalf("read utoc: %v", err)
	}

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("StripCompanionPaks() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 || result.Succeeded[0] != entry.ID {
		t.Fatalf("Succeeded = %v, want [%s]", result.Succeeded, entry.ID)
	}
	body, err := os.ReadFile(filepath.Join(root, "skins", "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read rewritten pak: %v", err)
	}
	if string(body) != "rewritten-companion" {
		t.Errorf("primary content = %q, want rewritten-companion", body)
	}
	afterUTOC, err := os.ReadFile(filepath.Join(root, "skins", "Hero_9999999_P.utoc"))
	if err != nil {
		t.Fatalf("read utoc after rewrite: %v", err)
	}
	if string(afterUTOC) != string(originalUTOC) {
		t.Fatal("utoc changed during companion PAK cleanup")
	}
}

func TestStripCompanionPaksPreservesDisabledPrimary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", true)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedCompanionCaller{
		listingByPak: map[string]string{"Hero_9999999_P.pak_crateoff": metadataListing()},
	}

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("StripCompanionPaks() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 {
		t.Fatalf("Succeeded = %v, want the disabled id", result.Succeeded)
	}
	if _, err := os.Stat(filepath.Join(root, "Hero_9999999_P.pak_crateoff")); err != nil {
		t.Fatalf("disabled primary missing after cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Hero_9999999_P.pak")); !os.IsNotExist(err) {
		t.Fatal("cleanup wrote an enabled .pak next to a disabled primary")
	}
}

func TestStripCompanionPaksSkipsCleanPak(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedCompanionCaller{
		listingByPak: map[string]string{"Hero_9999999_P.pak": `{"files":[{"path":"Audio/sound.bnk","size":4}]}`},
	}

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("StripCompanionPaks() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 {
		t.Fatalf("Succeeded = %v, want the clean id", result.Succeeded)
	}
	if caller.createFileCount != 0 {
		t.Fatalf("create_pak called %d times, want 0 for a clean listing", caller.createFileCount)
	}
	body, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read primary: %v", err)
	}
	if string(body) != "live-Hero_9999999_P.pak" {
		t.Errorf("clean bundle was rewritten: %q", body)
	}
}

func TestStripCompanionPaksRollsBackFailedRebuild(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedCompanionCaller{
		listingByPak: map[string]string{"Hero_9999999_P.pak": metadataListing()},
		failCreate:   true,
	}
	original, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("StripCompanionPaks() error = %v, want nil with a per-mod failure", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want one failure", result.Failed)
	}
	after, err := os.ReadFile(filepath.Join(root, "Hero_9999999_P.pak"))
	if err != nil {
		t.Fatalf("read after failed cleanup: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("primary changed after failed rebuild: %q", after)
	}
}

func TestStripCompanionPaksKeepsHybridRawFiles(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	caller := &scriptedCompanionCaller{
		listingByPak:  map[string]string{"Hero_9999999_P.pak": hybridListing()},
		extractWrites: map[string]string{"sound.bnk": "raw"},
	}

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, caller, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("StripCompanionPaks() error = %v, want nil", err)
	}
	if len(result.Succeeded) != 1 {
		t.Fatalf("Succeeded = %v, want the hybrid id", result.Succeeded)
	}
	if caller.createFileCount != 1 {
		t.Fatalf("create_pak called %d times, want 1", caller.createFileCount)
	}
}

func TestStripCompanionPaksRejectsEmptySelection(t *testing.T) {
	// Act
	_, err := StripCompanionPaks(t.TempDir(), nil, &scriptedCompanionCaller{}, nil, nil)

	// Assert
	if err == nil {
		t.Fatal("StripCompanionPaks() error = nil, want an error for an empty selection")
	}
}

func TestStripCompanionPaksReportsCancel(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeIoStoreBundle(t, root, "", "Hero_9999999_P", false)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	entry := library.Entries[0]
	cancel := make(chan struct{})
	close(cancel)

	// Act
	result, err := StripCompanionPaks(root, []string{entry.ID}, &scriptedCompanionCaller{
		listingByPak: map[string]string{"Hero_9999999_P.pak": metadataListing()},
	}, nil, cancel)

	// Assert
	if !errors.Is(err, ErrCompanionCleanupCancelled) {
		t.Fatalf("StripCompanionPaks() error = %v, want ErrCompanionCleanupCancelled", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want the cancelled id", result.Failed)
	}
}
