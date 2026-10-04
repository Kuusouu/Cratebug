package backup

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Builds a library covering every report category: an enabled classic mod, a
// disabled classic mod, a complete IoStore bundle, an IoStore bundle missing
// its .ucas sidecar, and an orphaned sidecar group.
func mixedLibrary(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Hero_123_P.pak"), []byte("classic-enabled"))
	writeFile(t, filepath.Join(root, "Old_456_P.pak_crateoff"), []byte("classic-disabled"))
	writeFile(t, filepath.Join(root, "skins", "Skin_1_P.pak"), []byte("iostore-primary"))
	writeFile(t, filepath.Join(root, "skins", "Skin_1_P.utoc"), []byte("iostore-toc"))
	writeFile(t, filepath.Join(root, "skins", "Skin_1_P.ucas"), []byte("iostore-cas"))
	writeFile(t, filepath.Join(root, "skins", "Broken_2_P.pak"), []byte("iostore-incomplete"))
	writeFile(t, filepath.Join(root, "skins", "Broken_2_P.utoc"), []byte("iostore-incomplete-toc"))
	writeFile(t, filepath.Join(root, "leftovers", "Orphan_P.utoc"), []byte("orphan-toc"))
	return root
}

func readZipEntries(t *testing.T, zipPath string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := make(map[string]string)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			entries[file.Name] = "<dir>"
			continue
		}
		contents, err := readZipFile(file)
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = string(contents)
	}
	return entries
}

func TestCountsLibrarySeparatesFormatsFromInvalid(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	// Act.
	counts := CountsLibrary(library.Entries)

	// Assert.
	if counts.Mods != 5 || counts.Classic != 2 || counts.IoStore != 1 || counts.Invalid != 2 {
		t.Errorf("counts = %+v, want 5 mods (2 classic, 1 iostore, 2 invalid)", counts)
	}
}

func TestCreateArchivesTreeAndMetadata(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	metadataBytes := []byte(`{"schemaVersion":1}`)
	writeFile(t, metadataPath, metadataBytes)
	dest := filepath.Join(t.TempDir(), "backup.zip")
	var progress []Progress

	// Act.
	result, err := Create(context.Background(), root, metadataPath, dest, func(p Progress) {
		progress = append(progress, p)
	})

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if result.DestinationPath == "" || !result.MetadataIncluded {
		t.Errorf("result = %+v, want a destination path with metadata included", result)
	}
	if result.Counts.Mods != 5 || result.Counts.Classic != 2 || result.Counts.IoStore != 1 || result.Counts.Invalid != 2 {
		t.Errorf("counts = %+v, want 5 mods (2 classic, 1 iostore, 2 invalid)", result.Counts)
	}
	entries := readZipEntries(t, dest)
	want := map[string]string{
		"Hero_123_P.pak":          "classic-enabled",
		"Old_456_P.pak_crateoff":  "classic-disabled",
		"skins/Skin_1_P.pak":      "iostore-primary",
		"skins/Skin_1_P.utoc":     "iostore-toc",
		"skins/Skin_1_P.ucas":     "iostore-cas",
		"skins/Broken_2_P.pak":    "iostore-incomplete",
		"skins/Broken_2_P.utoc":   "iostore-incomplete-toc",
		"leftovers/Orphan_P.utoc": "orphan-toc",
		"metadata.json":           `{"schemaVersion":1}`,
	}
	for name, contents := range want {
		if entries[name] != contents {
			t.Errorf("zip entry %q = %q, want %q", name, entries[name], contents)
		}
	}
	if len(progress) == 0 || progress[len(progress)-1].Total != len(want) {
		t.Errorf("progress events = %+v, want one per written entry totaling %d", progress, len(want))
	}
}

func TestCreateWithoutMetadataFile(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	dest := filepath.Join(t.TempDir(), "backup.zip")

	// Act.
	result, err := Create(context.Background(), root, filepath.Join(t.TempDir(), "missing.json"), dest, nil)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if result.MetadataIncluded {
		t.Errorf("MetadataIncluded = true, want false when the metadata file is missing")
	}
	entries := readZipEntries(t, dest)
	if _, found := entries["metadata.json"]; found {
		t.Errorf("zip contains metadata.json, want the tree without it")
	}
	if entries["Hero_123_P.pak"] != "classic-enabled" {
		t.Errorf("zip is missing library files: %v", entries)
	}
}

func TestCreateRejectsDestinationInsideLibrary(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	dest := filepath.Join(root, "backup.zip")

	// Act.
	_, err := Create(context.Background(), root, "", dest, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected an error for a destination inside the mod library")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("destination zip was created despite the rejection")
	}
}

func TestCreateRejectsMetadataNameCollision(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	writeFile(t, filepath.Join(root, "metadata.json"), []byte("stray"))
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	writeFile(t, metadataPath, []byte(`{"schemaVersion":1}`))
	dest := filepath.Join(t.TempDir(), "backup.zip")

	// Act.
	_, err := Create(context.Background(), root, metadataPath, dest, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected an error for the metadata.json collision")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("destination zip was created despite the collision")
	}
}

func TestCreateCancelledRemovesPartialDestination(t *testing.T) {
	// Arrange.
	root := mixedLibrary(t)
	dest := filepath.Join(t.TempDir(), "backup.zip")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act.
	_, err := Create(ctx, root, "", dest, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected a cancellation error")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("partial destination was left behind after cancellation")
	}
}

func TestCreateFailsOnMissingLibrary(t *testing.T) {
	// Arrange.
	dest := filepath.Join(t.TempDir(), "backup.zip")

	// Act.
	_, err := Create(context.Background(), filepath.Join(t.TempDir(), "missing-root"), "", dest, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected an error for a missing mod library")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("destination zip was created despite the scan failure")
	}
}
