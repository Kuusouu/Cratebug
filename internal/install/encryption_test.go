package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type installEncryptionCaller struct {
	encrypted map[string]bool
	failUTOC  string
	extracted []string
}

func (c *installEncryptionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "is_iostore_encrypted":
		path := params["file_path"].(string)
		body := fmt.Sprintf(`{"encrypted":%t}`, c.encrypted[filepath.Base(path)])
		return json.Unmarshal([]byte(body), result)
	case "extract_iostore":
		path := params["file_path"].(string)
		if filepath.Base(path) == c.failUTOC {
			return errors.New("fixture extraction failed")
		}
		c.extracted = append(c.extracted, filepath.Base(path))
		output := params["output_path"].(string)
		if err := os.WriteFile(filepath.Join(output, "asset.uasset"), []byte("asset"), testFilePermissions); err != nil {
			return err
		}
		return json.Unmarshal([]byte(`{"count":1}`), result)
	case "list_pak":
		return json.Unmarshal([]byte(`{"files":[]}`), result)
	case "create_mod_iostore":
		if params["obfuscate"] != true {
			return errors.New("install requested a clear rebuild")
		}
		output := params["output_path"].(string)
		for _, ext := range []string{".pak", ".utoc", ".ucas"} {
			if err := os.WriteFile(output+ext, []byte("encrypted"+ext), testFilePermissions); err != nil {
				return err
			}
		}
		return json.Unmarshal([]byte(`{"converted_count":1,"file_count":1}`), result)
	default:
		return fmt.Errorf("unexpected worker action %q", action)
	}
}

func stageEncryptionFixtures(t *testing.T, bundles map[string][]string) (*StagedSession, string) {
	t.Helper()
	source := t.TempDir()
	var paths []string
	for stem, extensions := range bundles {
		for _, ext := range extensions {
			path := filepath.Join(source, stem+ext)
			if err := os.WriteFile(path, []byte("source"+ext), testFilePermissions); err != nil {
				t.Fatal(err)
			}
			if ext == ".pak" {
				paths = append(paths, path)
			}
		}
	}
	session := &StagedSession{Dir: t.TempDir(), SourceFiles: paths}
	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	return session, source
}

func TestInstallEncryptionKeepsIndependentChoicesAndSourceFiles(t *testing.T) {
	// Arrange
	session, source := stageEncryptionFixtures(t, map[string][]string{
		"Clear_P":     {".pak", ".utoc", ".ucas"},
		"Preserve_P":  {".pak", ".utoc", ".ucas"},
		"Encrypted_P": {".pak", ".utoc", ".ucas"},
		"Excluded_P":  {".pak", ".utoc", ".ucas"},
		"Classic_P":   {".pak"},
	})
	root := t.TempDir()
	var items []ApplyItem
	for _, mod := range session.Mods {
		if mod.Stem == "Excluded_P" {
			continue
		}
		items = append(items, ApplyItem{
			ID: mod.ID, ModName: mod.DisplayName, DestinationFolder: "installed",
			Encrypt: mod.Stem == "Clear_P" || mod.Stem == "Encrypted_P",
		})
	}
	caller := &installEncryptionCaller{encrypted: map[string]bool{"Encrypted_P.utoc": true}}
	var progress []Progress

	// Act
	err := EncryptStagedMods(context.Background(), session, items, caller, func(p Progress) {
		progress = append(progress, p)
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(context.Background(), root, session, items, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(result.InstalledEntryIDs) != 4 {
		t.Fatalf("installed %d mods, want 4", len(result.InstalledEntryIDs))
	}
	if len(caller.extracted) != 1 || caller.extracted[0] != "Clear_P.utoc" {
		t.Fatalf("rebuilt %v, want only Clear_P.utoc", caller.extracted)
	}
	if len(progress) != 2 || progress[0].Phase != "encrypting" || progress[1].Current != 2 {
		t.Fatalf("unexpected encryption progress: %+v", progress)
	}
	for _, entry := range result.ReconciledLibrary.Entries {
		for _, path := range []string{entry.PrimaryPath, entry.Sidecars.UTOC, entry.Sidecars.UCAS} {
			if path == "" {
				continue
			}
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			want := "source" + filepath.Ext(path)
			if filepath.Base(entry.PrimaryPath) == "Clear_P.pak" {
				want = "encrypted" + filepath.Ext(path)
			}
			if string(body) != want {
				t.Errorf("installed %s = %q, want %q", path, body, want)
			}
		}
	}
	for _, mod := range session.Mods {
		for _, path := range mod.AllFiles {
			body, err := os.ReadFile(filepath.Join(source, filepath.Base(path)))
			if err != nil || string(body) != "source"+filepath.Ext(path) {
				t.Fatalf("source changed: %s", path)
			}
		}
	}
}

func TestInstallEncryptionRejectsUnsupportedChoicesBeforeRebuild(t *testing.T) {
	for _, extensions := range [][]string{{".pak"}, {".pak", ".utoc"}} {
		t.Run(strings.Join(extensions, "+"), func(t *testing.T) {
			// Arrange
			session, _ := stageEncryptionFixtures(t, map[string][]string{
				"Good_P": {".pak", ".utoc", ".ucas"}, "Invalid_P": extensions,
			})
			var items []ApplyItem
			for _, mod := range session.Mods {
				items = append(items, ApplyItem{ID: mod.ID, Encrypt: true})
			}
			caller := &installEncryptionCaller{}

			// Act
			err := EncryptStagedMods(context.Background(), session, items, caller, nil)

			// Assert
			if !errors.Is(err, mutation.ErrEncryptionIneligible) {
				t.Fatalf("error = %v, want ErrEncryptionIneligible", err)
			}
			if len(caller.extracted) != 0 {
				t.Fatal("an invalid selection started a rebuild")
			}
			preview, err := BuildPreview(t.TempDir(), session, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range preview.Items {
				if item.CanEncrypt != (item.ModName == "Good") {
					t.Errorf("%s encryption eligibility = %t", item.ModName, item.CanEncrypt)
				}
			}
		})
	}
}

func TestInstallEncryptionReportsFailedRebuild(t *testing.T) {
	// Arrange
	session, _ := stageEncryptionFixtures(t, map[string][]string{"Broken_P": {".pak", ".utoc", ".ucas"}})
	mod := session.Mods[0]
	caller := &installEncryptionCaller{failUTOC: "Broken_P.utoc"}

	// Act
	err := EncryptStagedMods(context.Background(), session, []ApplyItem{{ID: mod.ID, Encrypt: true}}, caller, nil)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "fixture extraction failed") {
		t.Fatalf("error = %v, want extraction failure", err)
	}
}

func TestInstallEncryptionHonorsCancellation(t *testing.T) {
	// Arrange
	session, _ := stageEncryptionFixtures(t, map[string][]string{"Clear_P": {".pak", ".utoc", ".ucas"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	caller := &installEncryptionCaller{}

	// Act
	err := EncryptStagedMods(ctx, session, []ApplyItem{{ID: session.Mods[0].ID, Encrypt: true}}, caller, nil)

	// Assert
	if !errors.Is(err, context.Canceled) || len(caller.extracted) != 0 {
		t.Fatalf("error = %v, rebuilt = %v", err, caller.extracted)
	}
}
