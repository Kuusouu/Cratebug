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
	failPak   string
	pakFixed  []string
}

func (c *installEncryptionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "is_iostore_encrypted":
		path := params["file_path"].(string)
		// Staged direct-encrypt copies are named bundle.utoc; the preceding
		// pak_fixer call encrypts them.
		if filepath.Base(path) == "bundle.utoc" {
			encrypted := c.failPak == ""
			return json.Unmarshal([]byte(fmt.Sprintf(`{"encrypted":%t}`, encrypted)), result)
		}
		body := fmt.Sprintf(`{"encrypted":%t}`, c.encrypted[filepath.Base(path)])
		return json.Unmarshal([]byte(body), result)
	case "pak_fixer":
		path := params["file_path"].(string)
		if params["obfuscate"] != true {
			return errors.New("install requested a clear rewrite")
		}
		// The staged bundle keeps its live stem; fail the one under test by
		// matching the live utoc stem recorded in failPak.
		if c.failPak != "" {
			return errors.New("fixture direct encryption failed")
		}
		c.pakFixed = append(c.pakFixed, filepath.Base(path))
		base := path[:len(path)-len(filepath.Ext(path))]
		for _, ext := range []string{".pak", ".utoc", ".ucas"} {
			if err := os.WriteFile(base+ext, []byte("encrypted"+ext), testFilePermissions); err != nil {
				return err
			}
		}
		return json.Unmarshal([]byte(`{"total":1,"fixed":0,"already_clean":1,"failed":0,"containers_encrypted":1,"containers_already_encrypted":0,"containers_failed":0}`), result)
	case "extract_iostore":
		return fmt.Errorf("unexpected worker action %q: staged encrypt must not extract", action)
	case "list_pak":
		return json.Unmarshal([]byte(`{"files":[]}`), result)
	case "create_mod_iostore":
		return fmt.Errorf("unexpected worker action %q: staged encrypt must not rebuild", action)
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
	if len(caller.pakFixed) != 1 {
		t.Fatalf("encrypted %v, want one direct encryption", caller.pakFixed)
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
			if len(caller.pakFixed) != 0 {
				t.Fatal("an invalid selection started an encryption")
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
	caller := &installEncryptionCaller{failPak: "Broken_P.utoc"}

	// Act
	err := EncryptStagedMods(context.Background(), session, []ApplyItem{{ID: mod.ID, Encrypt: true}}, caller, nil)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "fixture direct encryption failed") {
		t.Fatalf("error = %v, want direct encryption failure", err)
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
	if !errors.Is(err, context.Canceled) || len(caller.pakFixed) != 0 {
		t.Fatalf("error = %v, encrypted = %v", err, caller.pakFixed)
	}
}
