package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type mockGameRunningChecker struct {
	running bool
	err     error
}

func (m mockGameRunningChecker) IsGameRunning() (bool, error) {
	return m.running, m.err
}

const (
	testDirPermissions  = 0755
	testFilePermissions = 0644
)

func createZipArchive(t *testing.T, targetPath string, files map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(targetPath), testDirPermissions); err != nil {
		t.Fatalf("create zip parent dir: %v", err)
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	if err := os.WriteFile(targetPath, buf.Bytes(), testFilePermissions); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
}

func createTarArchive(t *testing.T, targetPath string, entries []tarEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(targetPath), testDirPermissions); err != nil {
		t.Fatalf("create tar parent dir: %v", err)
	}

	f, err := os.Create(targetPath)
	if err != nil {
		t.Fatalf("create tar file: %v", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.content)),
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %q: %v", e.name, err)
		}
		if len(e.content) > 0 {
			if _, err := tw.Write(e.content); err != nil {
				t.Fatalf("write tar content %q: %v", e.name, err)
			}
		}
	}
}

type tarEntry struct {
	name     string
	content  []byte
	mode     int64
	typeflag byte
	linkname string
}

func TestExtractArchive_ZipSuccess(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "mod.zip")
	destDir := filepath.Join(tempDir, "extracted")

	createZipArchive(t, zipPath, map[string][]byte{
		"Hulk_P.pak":   []byte("pak-content"),
		"Hulk_P.utoc":  []byte("utoc-content"),
		"Hulk_P.ucas":  []byte("ucas-content"),
		"sub/info.txt": []byte("some text"),
	})

	// Act
	err := ExtractArchive(context.Background(), zipPath, destDir)

	// Assert
	if err != nil {
		t.Fatalf("ExtractArchive failed: %v", err)
	}

	for _, expectedFile := range []string{"Hulk_P.pak", "Hulk_P.utoc", "Hulk_P.ucas", "sub/info.txt"} {
		path := filepath.Join(destDir, filepath.FromSlash(expectedFile))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected extracted file missing: %s", path)
		}
	}
}

func TestExtractArchive_RejectsZipSlip(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "malicious.zip")
	destDir := filepath.Join(tempDir, "extracted")

	createZipArchive(t, zipPath, map[string][]byte{
		"../../escaped.txt": []byte("evil payload"),
	})

	// Act
	err := ExtractArchive(context.Background(), zipPath, destDir)

	// Assert
	if err == nil {
		t.Fatal("expected ExtractArchive to fail on Zip Slip attempt, but it succeeded")
	}

	escapedFile := filepath.Join(tempDir, "escaped.txt")
	if _, err := os.Stat(escapedFile); err == nil {
		t.Fatalf("escaped file was written outside destination directory: %s", escapedFile)
	}
}

func TestExtractArchive_RejectsSymlinks(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "symlink.tar")
	destDir := filepath.Join(tempDir, "extracted")

	createTarArchive(t, tarPath, []tarEntry{
		{
			name:     "evil_symlink",
			typeflag: tar.TypeSymlink,
			linkname: "/etc/passwd",
			mode:     0777,
		},
	})

	// Act
	err := ExtractArchive(context.Background(), tarPath, destDir)

	// Assert
	if err == nil {
		t.Fatal("expected ExtractArchive to reject symlink, but it succeeded")
	}
	if !strings.Contains(err.Error(), "unsafe link") {
		t.Errorf("expected error message to mention unsafe link, got: %v", err)
	}
}

func TestStageFiles_MixedArchivesAndDirectFiles(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "mod1.zip")
	createZipArchive(t, zipPath, map[string][]byte{
		"SpiderMan_P.pak":  []byte("spiderman-pak"),
		"SpiderMan_P.utoc": []byte("spiderman-utoc"),
		"SpiderMan_P.ucas": []byte("spiderman-ucas"),
	})

	directPak := filepath.Join(tempDir, "IronMan_P.pak")
	if err := os.WriteFile(directPak, []byte("ironman-pak"), testFilePermissions); err != nil {
		t.Fatalf("create direct pak: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{zipPath, directPak})
	if err != nil {
		t.Fatalf("create staging session: %v", err)
	}
	defer session.Cleanup()

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}

	if len(session.Mods) != 2 {
		t.Fatalf("expected 2 discovered mods, got %d", len(session.Mods))
	}

	var foundSpiderMan, foundIronMan bool
	for _, m := range session.Mods {
		if strings.Contains(m.DisplayName, "SpiderMan") {
			foundSpiderMan = true
			if m.BundleFormat != discovery.BundleFormatIoStore {
				t.Errorf("SpiderMan expected BundleFormatIoStore, got %v", m.BundleFormat)
			}
		}
		if strings.Contains(m.DisplayName, "IronMan") {
			foundIronMan = true
			if m.BundleFormat != discovery.BundleFormatClassic {
				t.Errorf("IronMan expected BundleFormatClassic, got %v", m.BundleFormat)
			}
		}
	}

	if !foundSpiderMan || !foundIronMan {
		t.Errorf("expected SpiderMan and IronMan mods discovered, got: %+v", session.Mods)
	}
}

func TestStageFiles_AutoDiscoversCompanionSidecars(t *testing.T) {
	// Arrange: A folder on disk containing a full IoStore bundle (.pak, .utoc, .ucas)
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "LooseMod")
	if err := os.MkdirAll(modDir, testDirPermissions); err != nil {
		t.Fatalf("create mod dir: %v", err)
	}

	pakPath := filepath.Join(modDir, "Lagoona_9999999_P.pak")
	utocPath := filepath.Join(modDir, "Lagoona_9999999_P.utoc")
	ucasPath := filepath.Join(modDir, "Lagoona_9999999_P.ucas")

	if err := os.WriteFile(pakPath, []byte("pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	if err := os.WriteFile(utocPath, []byte("utoc-data"), testFilePermissions); err != nil {
		t.Fatalf("write utoc: %v", err)
	}
	if err := os.WriteFile(ucasPath, []byte("ucas-data"), testFilePermissions); err != nil {
		t.Fatalf("write ucas: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{pakPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act: User selects ONLY the .pak file for staging
	err = session.StageFiles(context.Background(), nil)

	// Assert: It should automatically detect and stage the .utoc and .ucas companions
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}
	if len(session.Mods) != 1 {
		t.Fatalf("expected 1 mod discovered, got %d", len(session.Mods))
	}

	mod := session.Mods[0]
	if mod.BundleFormat != discovery.BundleFormatIoStore {
		t.Errorf("expected BundleFormatIoStore, got %v", mod.BundleFormat)
	}
	if mod.Sidecars.UTOC == "" || mod.Sidecars.UCAS == "" {
		t.Errorf("expected both UTOC and UCAS sidecars populated, got %+v", mod.Sidecars)
	}
	if len(mod.AllFiles) != 3 {
		t.Errorf("expected 3 files in bundle, got %d: %v", len(mod.AllFiles), mod.AllFiles)
	}
}

func TestStageFiles_CompanionDiscovery_CaseInsensitiveMatch(t *testing.T) {
	// Arrange: sidecars use uppercase and mixed-case extensions
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "LooseMod")
	if err := os.MkdirAll(modDir, testDirPermissions); err != nil {
		t.Fatalf("create mod dir: %v", err)
	}

	pakPath := filepath.Join(modDir, "Lagoona_9999999_P.pak")
	if err := os.WriteFile(pakPath, []byte("pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Lagoona_9999999_P.UTOC"), []byte("utoc-data"), testFilePermissions); err != nil {
		t.Fatalf("write utoc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Lagoona_9999999_P.Ucas"), []byte("ucas-data"), testFilePermissions); err != nil {
		t.Fatalf("write ucas: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{pakPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert: uppercase/mixed-case sidecar extensions are still matched
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}
	if len(session.Mods) != 1 {
		t.Fatalf("expected 1 mod discovered, got %d", len(session.Mods))
	}
	mod := session.Mods[0]
	if mod.BundleFormat != discovery.BundleFormatIoStore {
		t.Errorf("expected BundleFormatIoStore, got %v", mod.BundleFormat)
	}
	if len(mod.AllFiles) != 3 {
		t.Errorf("expected 3 files in bundle, got %d: %v", len(mod.AllFiles), mod.AllFiles)
	}
}

func TestStageFiles_CompanionDiscovery_ExcludesDifferentStem(t *testing.T) {
	// Arrange: an unrelated mod's primary sits in the same source directory
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "LooseMod")
	if err := os.MkdirAll(modDir, testDirPermissions); err != nil {
		t.Fatalf("create mod dir: %v", err)
	}

	pakPath := filepath.Join(modDir, "Foo_P.pak")
	if err := os.WriteFile(pakPath, []byte("pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Foo_P.utoc"), []byte("utoc-data"), testFilePermissions); err != nil {
		t.Fatalf("write utoc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Foo_P.ucas"), []byte("ucas-data"), testFilePermissions); err != nil {
		t.Fatalf("write ucas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Bar_P.pak"), []byte("unrelated-pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write unrelated pak: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{pakPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act: User selects ONLY Foo_P.pak
	err = session.StageFiles(context.Background(), nil)

	// Assert: the unrelated Bar_P.pak is not pulled in as a companion
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}
	if len(session.Mods) != 1 {
		t.Fatalf("expected 1 mod discovered, got %d: %+v", len(session.Mods), session.Mods)
	}
	if len(session.Mods[0].AllFiles) != 3 {
		t.Errorf("expected 3 files (pak+utoc+ucas), got %d: %v", len(session.Mods[0].AllFiles), session.Mods[0].AllFiles)
	}
}

func TestStageFiles_CompanionDiscovery_PicksUpDisabledSuffixVariant(t *testing.T) {
	// Arrange: an enabled primary sits alongside a disabled-suffix variant of itself
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "LooseMod")
	if err := os.MkdirAll(modDir, testDirPermissions); err != nil {
		t.Fatalf("create mod dir: %v", err)
	}

	pakPath := filepath.Join(modDir, "Foo_9999999_P.pak")
	if err := os.WriteFile(pakPath, []byte("pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Foo_9999999_P.pak_disabled"), []byte("disabled-pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write disabled variant: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{pakPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert: both variants staged, so discovery reports them as ambiguous primaries -
	// proof the .pak_disabled companion was actually copied, not just the .pak itself.
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}
	if len(session.Mods) != 2 {
		t.Fatalf("expected 2 mods (enabled + disabled variant), got %d: %+v", len(session.Mods), session.Mods)
	}
	for _, mod := range session.Mods {
		found := false
		for _, issue := range mod.Issues {
			if issue.Code == discovery.IssueAmbiguousPrimary {
				found = true
			}
		}
		if !found {
			t.Errorf("expected mod %q to report an ambiguous-primary issue, got %+v", mod.DisplayName, mod.Issues)
		}
	}
}

func TestStageFiles_CompanionCopyFailure_SurfacesIssueInsteadOfSilentClassic(t *testing.T) {
	// Arrange: a .pak whose intended .utoc/.ucas companions cannot be copied
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "LooseMod")
	if err := os.MkdirAll(modDir, testDirPermissions); err != nil {
		t.Fatalf("create mod dir: %v", err)
	}

	pakPath := filepath.Join(modDir, "Foo_P.pak")
	if err := os.WriteFile(pakPath, []byte("pak-data"), testFilePermissions); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Foo_P.utoc"), []byte("utoc-data"), testFilePermissions); err != nil {
		t.Fatalf("write utoc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "Foo_P.ucas"), []byte("ucas-data"), testFilePermissions); err != nil {
		t.Fatalf("write ucas: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{pakPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Obstruct both sidecar destinations with directories so copyRegularFile fails for
	// each companion, simulating a locked-file/permission-denied failure portably.
	looseDir := filepath.Join(session.Dir, "loose")
	if err := os.MkdirAll(filepath.Join(looseDir, "Foo_P.utoc"), testDirPermissions); err != nil {
		t.Fatalf("obstruct utoc destination: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(looseDir, "Foo_P.ucas"), testDirPermissions); err != nil {
		t.Fatalf("obstruct ucas destination: %v", err)
	}

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert: staging still succeeds best-effort, but the mod is flagged rather than
	// silently reported as a plain Classic bundle.
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}
	if len(session.Mods) != 1 {
		t.Fatalf("expected 1 mod discovered, got %d", len(session.Mods))
	}

	mod := session.Mods[0]
	if mod.BundleFormat != discovery.BundleFormatClassic {
		t.Errorf("expected BundleFormatClassic since neither sidecar landed, got %v", mod.BundleFormat)
	}
	if len(mod.Issues) != 2 {
		t.Fatalf("expected 2 companion-copy-failed issues, got %d: %+v", len(mod.Issues), mod.Issues)
	}
	for _, issue := range mod.Issues {
		if issue.Code != issueCompanionCopyFailed {
			t.Errorf("expected issue code %q, got %q", issueCompanionCopyFailed, issue.Code)
		}
	}

	// The failure must also reach the preview the user actually sees.
	preview, err := BuildPreview(tempDir, session, "", nil)
	if err != nil {
		t.Fatalf("BuildPreview failed: %v", err)
	}
	if len(preview.Items) != 1 || len(preview.Items[0].Issues) != 2 {
		t.Fatalf("expected the copy-failure issues to reach the preview item, got %+v", preview.Items)
	}
}

func TestBuildPreview_CollisionDetection(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	existingPak := filepath.Join(modRoot, "Characters", "Hulk", "Hulk_P.pak")
	if err := os.MkdirAll(filepath.Dir(existingPak), testDirPermissions); err != nil {
		t.Fatalf("create existing mod dir: %v", err)
	}
	if err := os.WriteFile(existingPak, []byte("existing"), testFilePermissions); err != nil {
		t.Fatalf("write existing pak: %v", err)
	}

	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "hulk_mod.zip")
	createZipArchive(t, zipPath, map[string][]byte{
		"Hulk_P.pak": []byte("new-hulk"),
	})

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{zipPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatalf("stage files: %v", err)
	}

	// Act - Collision case
	previewWithCollision, err := BuildPreview(modRoot, session, "Characters/Hulk", nil)
	if err != nil {
		t.Fatalf("BuildPreview failed: %v", err)
	}

	// Assert collision
	if len(previewWithCollision.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(previewWithCollision.Items))
	}
	if !previewWithCollision.Items[0].Collision.HasCollision {
		t.Errorf("expected collision to be detected for Hulk_P.pak in Characters/Hulk")
	}

	// Act - No collision case (different folder)
	previewNoCollision, err := BuildPreview(modRoot, session, "Characters/Thor", nil)
	if err != nil {
		t.Fatalf("BuildPreview failed: %v", err)
	}

	// Assert no collision
	if previewNoCollision.Items[0].Collision.HasCollision {
		t.Errorf("expected no collision for Hulk_P.pak in Characters/Thor")
	}
}

func TestApply_TransactionalSuccess(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "mod.zip")
	createZipArchive(t, zipPath, map[string][]byte{
		"Storm_9999999_P.pak":  []byte("storm-pak"),
		"Storm_9999999_P.utoc": []byte("storm-utoc"),
		"Storm_9999999_P.ucas": []byte("storm-ucas"),
	})

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{zipPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatalf("stage files: %v", err)
	}

	items := []ApplyItem{
		{
			ID:                session.Mods[0].ID,
			ModName:           "Storm",
			DestinationFolder: "Characters/Storm",
			Overwrite:         false,
		},
	}

	checker := mockGameRunningChecker{running: false}

	// Act
	result, err := Apply(context.Background(), modRoot, session, items, checker)

	// Assert
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if len(result.InstalledEntryIDs) != 1 {
		t.Fatalf("expected 1 installed entry ID, got %d", len(result.InstalledEntryIDs))
	}

	installedPak := filepath.Join(modRoot, "Characters", "Storm", "Storm_9999999_P.pak")
	installedUtoc := filepath.Join(modRoot, "Characters", "Storm", "Storm_9999999_P.utoc")
	installedUcas := filepath.Join(modRoot, "Characters", "Storm", "Storm_9999999_P.ucas")

	for _, p := range []string{installedPak, installedUtoc, installedUcas} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected installed file missing: %s", p)
		}
	}
}

func TestApply_GameRunningBlock(t *testing.T) {
	// Arrange
	modRoot := t.TempDir()
	tempDir := t.TempDir()
	directPak := filepath.Join(tempDir, "Venom_P.pak")
	if err := os.WriteFile(directPak, []byte("venom"), testFilePermissions); err != nil {
		t.Fatalf("write direct pak: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{directPak})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatalf("stage files: %v", err)
	}

	items := []ApplyItem{
		{
			ID:                session.Mods[0].ID,
			ModName:           "Venom",
			DestinationFolder: "",
			Overwrite:         false,
		},
	}

	checker := mockGameRunningChecker{running: true}

	// Act
	_, err = Apply(context.Background(), modRoot, session, items, checker)

	// Assert
	if err != mutation.ErrGameRunning {
		t.Fatalf("expected ErrGameRunning, got: %v", err)
	}

	destPak := filepath.Join(modRoot, "Venom_P.pak")
	if _, err := os.Stat(destPak); err == nil {
		t.Fatalf("file should not have been installed while game running")
	}
}

func TestApply_OverwriteRollbackOnFailure(t *testing.T) {
	// Arrange: put an existing file that will be overwritten by the first item,
	// then set up a second item whose staged source is missing so Apply fails
	// partway through, exercising the actual rollback-restore path.
	modRoot := t.TempDir()
	existingPak := filepath.Join(modRoot, "Loki_P.pak")
	originalContent := []byte("original-loki-content")
	if err := os.WriteFile(existingPak, originalContent, testFilePermissions); err != nil {
		t.Fatalf("write existing pak: %v", err)
	}

	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "loki.zip")
	createZipArchive(t, zipPath, map[string][]byte{
		"Loki_P.pak": []byte("new-loki-content"),
	})

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{zipPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatalf("stage files: %v", err)
	}

	// Inject a second fake staged mod whose source file does not exist on disk,
	// guaranteeing Apply fails after successfully overwriting the first file.
	session.Mods = append(session.Mods, StagedMod{
		ID:                  "staged-mod-missing",
		RelativePrimaryPath: "nonexistent/Ghost_P.pak",
		BundleFormat:        discovery.BundleFormatClassic,
		DisplayName:         "Ghost",
		Stem:                "Ghost_P",
		AllFiles:            []string{"nonexistent/Ghost_P.pak"},
	})

	items := []ApplyItem{
		{
			ID:                session.Mods[0].ID,
			ModName:           "Loki",
			DestinationFolder: "",
			Overwrite:         true,
		},
		{
			ID:                "staged-mod-missing",
			ModName:           "Ghost",
			DestinationFolder: "",
			Overwrite:         false,
		},
	}

	checker := mockGameRunningChecker{running: false}

	// Act
	_, err = Apply(context.Background(), modRoot, session, items, checker)

	// Assert
	if err == nil {
		t.Fatal("expected Apply to fail due to missing staged source file")
	}

	// The original overwritten file must be restored by rollback.
	content, readErr := os.ReadFile(existingPak)
	if readErr != nil {
		t.Fatalf("read original file after rollback: %v", readErr)
	}
	if !bytes.Equal(content, originalContent) {
		t.Fatalf("expected original content %q restored after rollback, got %q", string(originalContent), string(content))
	}

	// The second mod's file must not exist (rollback should have removed it,
	// but it was never written because the copy failed).
	ghostPak := filepath.Join(modRoot, "Ghost_P.pak")
	if _, err := os.Stat(ghostPak); err == nil {
		t.Fatalf("expected Ghost_P.pak to not exist after rollback, but it does")
	}
}

func TestValidateModName_ReservedDeviceNames(t *testing.T) {
	// Arrange
	reservedNames := []string{"CON", "con", "PRN", "aux", "NUL", "COM1", "com9", "LPT1", "lpt9"}

	// Act & Assert
	for _, name := range reservedNames {
		err := validateModName(name)
		if err == nil {
			t.Errorf("expected validateModName(%q) to fail for reserved device name, but it succeeded", name)
		}
	}
}

func TestBuildPreview_EscapingDestinationFolder(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "mod.zip")
	createZipArchive(t, zipPath, map[string][]byte{
		"Mod_P.pak": []byte("content"),
	})

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{zipPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	if err := session.StageFiles(context.Background(), nil); err != nil {
		t.Fatalf("stage files: %v", err)
	}

	// Act
	_, err = BuildPreview(tempDir, session, "../outside", nil)

	// Assert
	if err == nil {
		t.Fatal("expected BuildPreview to fail for escaping destination folder, but it succeeded")
	}
	if !strings.Contains(err.Error(), "escapes mod root") {
		t.Errorf("expected error message to mention escaping mod root, got: %v", err)
	}
}

func TestSessionManager_CleanupAll(t *testing.T) {
	// Arrange
	sm := NewSessionManager()
	session1, err := sm.CreateSession([]string{"file1.zip"})
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}
	session2, err := sm.CreateSession([]string{"file2.zip"})
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	dir1 := session1.Dir
	dir2 := session2.Dir

	// Act
	if err := sm.CleanupAll(); err != nil {
		t.Fatalf("CleanupAll failed: %v", err)
	}

	// Assert
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Errorf("expected session 1 dir to be removed, but still exists")
	}
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Errorf("expected session 2 dir to be removed, but still exists")
	}
	if sm.GetSession(session1.ID) != nil {
		t.Errorf("expected session 1 to be removed from manager")
	}
	if sm.GetSession(session2.ID) != nil {
		t.Errorf("expected session 2 to be removed from manager")
	}
}

func TestDetermineFinalStem_NoDuplicateSuffix(t *testing.T) {
	// Arrange
	stagedMod := StagedMod{
		DisplayName: "Hero",
		Stem:        "Hero_P",
	}

	// Act & Assert
	if stem := determineFinalStem(stagedMod, "Hero"); stem != "Hero_P" {
		t.Errorf("expected unchanged name to preserve original stem 'Hero_P', got %q", stem)
	}
	if stem := determineFinalStem(stagedMod, "Hero New"); stem != "Hero New_P" {
		t.Errorf("expected 'Hero New_P', got %q", stem)
	}
	if stem := determineFinalStem(stagedMod, "Hero New_P"); stem != "Hero New_P" {
		t.Errorf("expected 'Hero New_P' without duplicate _P, got %q", stem)
	}
}

func TestStageFiles_IncompleteMultiVolumeRarFailsCleanly(t *testing.T) {
	// Arrange: a real rar archive (see testdata/README.md), the first volume of
	// a two-volume set. Cratebug reads a selected archive through a single
	// os.File reader (ExtractArchive), never multi-volume-aware sibling lookup,
	// so this is what a user selecting only the first part of a multi-volume
	// rar actually produces - not a rar containing zero mod files.
	rarPath := filepath.Join("testdata", "test.part01.rar")
	if _, err := os.Stat(rarPath); err != nil {
		t.Skipf("rar fixture unavailable: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{rarPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert: fails cleanly instead of silently succeeding with a truncated
	// extraction or leaving partial state behind.
	if err == nil {
		t.Fatalf("expected StageFiles to reject an incomplete multi-volume rar, got nil error; mods=%+v", session.Mods)
	}
	if !strings.Contains(err.Error(), "multi-volume") {
		t.Errorf("expected a multi-volume-related error, got: %v", err)
	}
	if _, statErr := os.Stat(session.Dir); statErr != nil {
		t.Errorf("expected staging dir to still exist for caller cleanup after a failed stage, stat err=%v", statErr)
	}
}

func TestStageFiles_GenuinelyEmptyRarRejectedCleanly(t *testing.T) {
	// Arrange: a real, valid, single-volume rar archive with zero entries
	// (see testdata/README.md).
	rarPath := filepath.Join("testdata", "empty.rar")
	if _, err := os.Stat(rarPath); err != nil {
		t.Skipf("rar fixture unavailable: %v", err)
	}

	sm := NewSessionManager()
	session, err := sm.CreateSession([]string{rarPath})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer session.Cleanup()

	// Act
	err = session.StageFiles(context.Background(), nil)

	// Assert: the archive decodes without error, same rejection as any other
	// archive with no mod files - not a crash, hang, or silent empty install.
	if err == nil {
		t.Fatalf("expected StageFiles to reject an empty rar, got nil error; mods=%+v", session.Mods)
	}
	if !strings.Contains(err.Error(), "no valid Marvel Rivals mod files") {
		t.Errorf("expected the standard no-mod-files rejection, got: %v", err)
	}
}
