package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

type stagedCompanionCaller struct {
	listing  string
	failList bool
}

func (s *stagedCompanionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "list_pak":
		if s.failList {
			return &uassettool.ToolError{Action: action, Message: "list failed"}
		}
		body := s.listing
		if body == "" {
			body = `{"files":[]}`
		}
		return json.Unmarshal([]byte(body), result)
	case "is_iostore_encrypted":
		return json.Unmarshal([]byte(`{"encrypted":false}`), result)
	case "create_pak":
		output, _ := params["output_path"].(string)
		return os.WriteFile(output, []byte("rewritten-companion"), 0o600)
	default:
		return nil
	}
}

func TestStripStagedCompanionPaksRewritesDirtyPrimary(t *testing.T) {
	// Arrange
	staging := t.TempDir()
	pak := filepath.Join(staging, "Hero_9999999_P.pak")
	if err := os.WriteFile(pak, []byte("dirty-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := &install.StagedSession{
		Dir: staging,
		Mods: []install.StagedMod{{
			ID:                  "mod-1",
			RelativePrimaryPath: "Hero_9999999_P.pak",
			DisplayName:         "Hero",
		}},
	}
	caller := &stagedCompanionCaller{
		listing: `{"files":[{"path":"chunknames","size":1}]}`,
	}

	// Act
	err := stripStagedCompanionPaks(session, []install.ApplyItem{{ID: "mod-1"}}, caller)

	// Assert
	if err != nil {
		t.Fatalf("stripStagedCompanionPaks() error = %v", err)
	}
	body, err := os.ReadFile(pak)
	if err != nil {
		t.Fatalf("read staged pak: %v", err)
	}
	if string(body) != "rewritten-companion" {
		t.Errorf("staged pak = %q, want rewritten-companion", body)
	}
}

func TestStripStagedCompanionPaksLeavesUnlistablePak(t *testing.T) {
	// Arrange
	staging := t.TempDir()
	pak := filepath.Join(staging, "Hero_9999999_P.pak")
	if err := os.WriteFile(pak, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := &install.StagedSession{
		Dir: staging,
		Mods: []install.StagedMod{{
			ID:                  "mod-1",
			RelativePrimaryPath: "Hero_9999999_P.pak",
			DisplayName:         "Hero",
		}},
	}

	// Act
	err := stripStagedCompanionPaks(session, []install.ApplyItem{{ID: "mod-1"}}, &stagedCompanionCaller{failList: true})

	// Assert
	if err != nil {
		t.Fatalf("stripStagedCompanionPaks() error = %v", err)
	}
	body, err := os.ReadFile(pak)
	if err != nil {
		t.Fatalf("read staged pak: %v", err)
	}
	if string(body) != "original" {
		t.Errorf("unlistable pak was rewritten: %q", body)
	}
}
func TestAppInstallLifecycle(t *testing.T) {
	if _, err := uassettool.ResolveExecutablePath(); err != nil {
		t.Skipf("pinned worker not found; run fetch-uassettool.ps1 first")
	}

	// Arrange
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	sourcePak := filepath.Join(sourceDir, "Punisher_P.pak")
	if err := os.WriteFile(sourcePak, []byte("punisher"), 0o600); err != nil {
		t.Fatalf("write source pak: %v", err)
	}

	app := testApp(t, false)

	// Act - Prepare
	preview, err := app.installs.PrepareInstall(modRoot, []string{sourcePak}, "Characters/Punisher")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}

	// Assert - Prepare
	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item in preview, got %d", len(preview.Items))
	}
	if preview.Items[0].ModName != "Punisher" {
		t.Errorf("expected ModName Punisher, got %q", preview.Items[0].ModName)
	}

	// Act - Apply
	applyItems := []install.ApplyItem{
		{
			ID:                preview.Items[0].ID,
			ModName:           "Punisher",
			DestinationFolder: "Characters/Punisher",
			Overwrite:         false,
		},
	}
	result, err := app.installs.ApplyInstall(modRoot, preview.SessionID, applyItems)

	// Assert - Apply
	if err != nil {
		t.Fatalf("ApplyInstall failed: %v", err)
	}
	if len(result.InstalledEntryIDs) != 1 {
		t.Fatalf("expected 1 installed entry ID, got %d", len(result.InstalledEntryIDs))
	}

	doc := app.settings.LoadMetadata().Document
	if _, ok := doc.FindModByScannerID(result.InstalledEntryIDs[0]); !ok {
		t.Fatal("freshly installed mod has no ModRecord")
	}

	destPak := filepath.Join(modRoot, "Characters", "Punisher", "Punisher_P.pak")
	if _, err := os.Stat(destPak); err != nil {
		t.Fatalf("expected installed file missing: %s", destPak)
	}
}

func TestApplyInstallReturnsResultWhenMetadataSaveFails(t *testing.T) {
	if _, err := uassettool.ResolveExecutablePath(); err != nil {
		t.Skipf("pinned worker not found; run fetch-uassettool.ps1 first")
	}

	// Arrange
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	sourcePak := filepath.Join(sourceDir, "Punisher_P.pak")
	if err := os.WriteFile(sourcePak, []byte("punisher"), 0o600); err != nil {
		t.Fatalf("write source pak: %v", err)
	}

	app := testApp(t, false)
	preview, err := app.installs.PrepareInstall(modRoot, []string{sourcePak}, "Characters/Punisher")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}
	app.meta = newMetadataGate(failingMetadataStore(t))

	// Act
	result, err := app.installs.ApplyInstall(modRoot, preview.SessionID, []install.ApplyItem{
		{
			ID:                preview.Items[0].ID,
			ModName:           "Punisher",
			DestinationFolder: "Characters/Punisher",
			Overwrite:         false,
		},
	})

	// Assert: Wails would hide ApplyResult if this returned a non-nil error.
	if err != nil {
		t.Fatalf("ApplyInstall() = %v, want nil after files landed", err)
	}
	if len(result.InstalledEntryIDs) != 1 {
		t.Fatalf("installedEntryIDs = %d, want 1", len(result.InstalledEntryIDs))
	}
	destPak := filepath.Join(modRoot, "Characters", "Punisher", "Punisher_P.pak")
	if _, err := os.Stat(destPak); err != nil {
		t.Fatalf("installed file missing after metadata save failure: %s", destPak)
	}
}

func TestAppInstallCancel(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	sourcePak := filepath.Join(sourceDir, "Groot_P.pak")
	if err := os.WriteFile(sourcePak, []byte("groot"), 0o600); err != nil {
		t.Fatalf("write source pak: %v", err)
	}

	app := testApp(t, false)

	preview, err := app.installs.PrepareInstall(modRoot, []string{sourcePak}, "")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}

	// Act - Cancel
	err = app.installs.CancelInstall(preview.SessionID)

	// Assert - Cancel
	if err != nil {
		t.Fatalf("CancelInstall failed: %v", err)
	}

	// Act - attempt apply after cancel
	applyItems := []install.ApplyItem{
		{
			ID:                preview.Items[0].ID,
			ModName:           "Groot",
			DestinationFolder: "",
			Overwrite:         false,
		},
	}
	_, err = app.installs.ApplyInstall(modRoot, preview.SessionID, applyItems)

	// Assert - apply fails because session was cancelled
	if err == nil {
		t.Fatalf("expected ApplyInstall to fail after CancelInstall, but it succeeded")
	}
}

func TestApplyInstallRecordsOnlyItsSessionNexusSource(t *testing.T) {
	if _, err := uassettool.ResolveExecutablePath(); err != nil {
		t.Skipf("pinned worker not found; run fetch-uassettool.ps1 first")
	}

	// Arrange: a Nexus download staged first, then a local file staged while
	// its preview is still open.
	modRoot := t.TempDir()
	sourceDir := t.TempDir()
	nexusPak := filepath.Join(sourceDir, "Storm_P.pak")
	localPak := filepath.Join(sourceDir, "Thor_P.pak")
	for _, path := range []string{nexusPak, localPak} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o600); err != nil {
			t.Fatalf("write source pak: %v", err)
		}
	}
	app := testApp(t, false)
	nexusPreview, err := app.installs.stage(modRoot, []string{nexusPak}, "", &nexusSource{ModID: 41, FileID: 7, Version: "1.2"})
	if err != nil {
		t.Fatalf("stage Nexus download: %v", err)
	}
	localPreview, err := app.installs.PrepareInstall(modRoot, []string{localPak}, "")
	if err != nil {
		t.Fatalf("PrepareInstall failed: %v", err)
	}
	applyItem := func(preview install.PreviewResult, name string) []install.ApplyItem {
		return []install.ApplyItem{{ID: preview.Items[0].ID, ModName: name}}
	}

	// Act
	localResult, err := app.installs.ApplyInstall(modRoot, localPreview.SessionID, applyItem(localPreview, "Thor"))
	if err != nil {
		t.Fatalf("apply local install: %v", err)
	}
	nexusResult, err := app.installs.ApplyInstall(modRoot, nexusPreview.SessionID, applyItem(nexusPreview, "Storm"))
	if err != nil {
		t.Fatalf("apply Nexus install: %v", err)
	}

	// Assert
	doc := app.settings.LoadMetadata().Document
	localModID, ok := doc.FindModByScannerID(localResult.InstalledEntryIDs[0])
	if !ok {
		t.Fatal("local install has no ModRecord")
	}
	if localRecord := doc.Mods[localModID]; localRecord.NexusModID != 0 {
		t.Fatalf("local install recorded Nexus mod %d, want none", localRecord.NexusModID)
	}
	nexusModID, ok := doc.FindModByScannerID(nexusResult.InstalledEntryIDs[0])
	if !ok {
		t.Fatal("Nexus install has no ModRecord")
	}
	if nexusRecord := doc.Mods[nexusModID]; nexusRecord.NexusModID != 41 || nexusRecord.NexusFileID != 7 || nexusRecord.NexusVersion != "1.2" {
		t.Fatalf("Nexus install source = %d/%d %q, want 41/7 \"1.2\"", nexusRecord.NexusModID, nexusRecord.NexusFileID, nexusRecord.NexusVersion)
	}
}
