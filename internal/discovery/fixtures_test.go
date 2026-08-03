package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const (
	fixtureContents    = "synthetic fixture\n"
	maximumFixtureSize = 64
)

var expectedFixturePaths = []string{
	"ambiguous/DualPrimary_9999999_P.pak",
	"ambiguous/DualPrimary_9999999_P.pak_crateoff",
	"classic/ClassicOnly_9999999_P.pak",
	"disabled/BentoDisabled_9999999_P.bak_bento",
	"disabled/CrateDisabled_9999999_P.pak_crateoff",
	"disabled/LegacyDisabled_9999999_P.pak_disabled",
	"duplicates/FolderA/SharedStem_9999999_P.pak",
	"duplicates/FolderB/SharedStem_9999999_P.pak",
	"enabled/Enabled_9999999_P.pak",
	"iostore/Complete_9999999_P.pak",
	"iostore/Complete_9999999_P.ucas",
	"iostore/Complete_9999999_P.utoc",
	"nested/Characters/Vanguard/Nested_9999999_P.pak",
	"orphan/OrphanUcas.ucas",
	"orphan/OrphanUtoc.utoc",
	"partial/MissingUcas_9999999_P.pak",
	"partial/MissingUcas_9999999_P.utoc",
	"partial/MissingUtoc_9999999_P.pak",
	"partial/MissingUtoc_9999999_P.ucas",
	"priority/!Example_9999999_P.pak",
	"priority/ExampleNoPriority.pak",
	"priority/Example_999999999_P.pak",
	"priority/Example_99999999_P.pak",
	"priority/Example_9999999_P.pak",
	"priority/Example_P.pak",
	"priority/Example_weirdpriority_P.pak",
}

// Ensures fixture files remain tiny, synthetic, and complete.
func TestFixtureLibraryIntegrity(t *testing.T) {
	t.Parallel()

	root := filepath.Join("testdata", "library")
	var actualPaths []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			t.Errorf("%s is not a regular file", path)
			return nil
		}
		if info.Size() > maximumFixtureSize {
			t.Errorf("%s is unexpectedly large: %d bytes", path, info.Size())
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(contents) != fixtureContents {
			t.Errorf("%s does not contain the standard synthetic payload", path)
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		actualPaths = append(actualPaths, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixture library: %v", err)
	}

	sort.Strings(actualPaths)
	if !reflect.DeepEqual(actualPaths, expectedFixturePaths) {
		t.Errorf("fixture inventory mismatch\nwant: %v\ngot:  %v", expectedFixturePaths, actualPaths)
	}
}
