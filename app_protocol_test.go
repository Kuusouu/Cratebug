package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
)

func TestCleanupLegacyNexusRestoresHandlerAndPreservesMetadata(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "nexus.key")
	if err := os.WriteFile(keyPath, []byte("unreadable-legacy-credential"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(dir, "metadata.json")
	store := metadata.NewStore(metadataPath)
	doc := metadata.Document{
		Settings: metadata.Settings{
			ModRoot: dir,
			Theme:   "dark",
			NexusProtocol: metadata.NexusProtocolSnapshot{
				Command:     `"C:\Apps\Vortex\Vortex.exe" "%1"`,
				DesktopFile: "vortex.desktop",
			},
		},
		Mods: map[string]metadata.ModRecord{
			"existing-mod": {ScannerID: "mod:skin", Tags: []string{"tag-1"}, NexusModID: 12, NexusFileID: 34, NexusVersion: "1.0"},
		},
		Tags: []metadata.Tag{{ID: "tag-1", Name: "Skin"}},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	registrar := urlscheme.NewForTest("cratebug-test-cleanup", `C:\Apps\Cratebug\Cratebug.exe`, nil)
	if _, err := registrar.Register(false); err != nil {
		t.Fatal(err)
	}

	// Act
	for range 2 {
		if err := cleanupLegacyNexus(store, registrar, keyPath); err != nil {
			t.Fatalf("cleanupLegacyNexus() = %v", err)
		}
	}

	// Assert
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy credential still exists: %v", err)
	}
	status, err := registrar.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Ownership != urlscheme.OwnershipOther || status.Snapshot.Command != doc.Settings.NexusProtocol.Command {
		t.Fatalf("handler = %+v, want the previous Vortex handler", status)
	}
	after, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("cleanup changed metadata, including existing tags or Nexus source IDs")
	}
}

func TestCleanupLegacyNexusLeavesUnownedHandlersAlone(t *testing.T) {
	for _, otherOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "no handler", true: "another manager"}[otherOwner], func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			metadataPath := filepath.Join(dir, "metadata.json")
			store := metadata.NewStore(metadataPath)
			registrar := urlscheme.NewForTest("cratebug-test-cleanup", `C:\Apps\Cratebug\Cratebug.exe`, nil)
			want := urlscheme.OwnershipNone
			foreign := urlscheme.Snapshot{Command: `"C:\Apps\Other\Other.exe" "%1"`}
			if otherOwner {
				if _, err := registrar.Register(false); err != nil {
					t.Fatal(err)
				}
				if err := registrar.Unregister(foreign); err != nil {
					t.Fatal(err)
				}
				want = urlscheme.OwnershipOther
			}

			// Act
			if err := cleanupLegacyNexus(store, registrar, filepath.Join(dir, "nexus.key")); err != nil {
				t.Fatal(err)
			}

			// Assert
			status, err := registrar.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.Ownership != want {
				t.Fatalf("ownership = %q, want %q", status.Ownership, want)
			}
			if otherOwner && status.Snapshot.Command != foreign.Command {
				t.Fatal("cleanup replaced another manager's handler")
			}
			if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
				t.Fatalf("cleanup created metadata for a fresh installation: %v", err)
			}
		})
	}
}

func TestCleanupLegacyNexusRestoresHandlerWhenCredentialRemovalFails(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "nexus.key")
	if err := os.Mkdir(keyPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyPath, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := metadata.NewStore(filepath.Join(dir, "metadata.json"))
	registrar := urlscheme.NewForTest("cratebug-test-cleanup", `C:\Apps\Cratebug\Cratebug.exe`, nil)
	if _, err := registrar.Register(false); err != nil {
		t.Fatal(err)
	}

	// Act
	err := cleanupLegacyNexus(store, registrar, keyPath)

	// Assert
	if err == nil {
		t.Fatal("cleanup succeeded despite failing to remove the legacy credential path")
	}
	status, statusErr := registrar.Status()
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Ownership != urlscheme.OwnershipNone {
		t.Fatal("credential removal failure prevented handler cleanup")
	}
	if _, err := os.Stat(filepath.Join(keyPath, "keep")); err != nil {
		t.Fatalf("cleanup removed unrelated contents: %v", err)
	}
}

func TestCleanupLegacyNexusLeavesAnotherCratebugInstallationAlone(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	registrar := urlscheme.NewForTest("cratebug-test-cleanup", filepath.Join(dir, "installed", "Cratebug.exe"), nil)
	if _, err := registrar.Register(false); err != nil {
		t.Fatal(err)
	}
	before, err := registrar.Status()
	if err != nil {
		t.Fatal(err)
	}
	registrar.Exe = filepath.Join(dir, "dev", "Cratebug.exe")

	// Act
	if err := cleanupLegacyNexus(metadata.NewStore(filepath.Join(dir, "metadata.json")), registrar, filepath.Join(dir, "nexus.key")); err != nil {
		t.Fatal(err)
	}

	// Assert
	after, err := registrar.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.Snapshot.Command != before.Snapshot.Command {
		t.Fatal("cleanup removed another Cratebug installation's handler")
	}
}
