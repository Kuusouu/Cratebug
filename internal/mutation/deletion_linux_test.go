//go:build linux

package mutation

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeleteModKeepsCaseDistinctFolderFiles(t *testing.T) {
	for _, test := range []struct {
		name      string
		primary   string
		selected  map[string]string
		remaining map[string]string
	}{
		{
			name:     "sidecars in another folder",
			primary:  "A/mod.pak",
			selected: map[string]string{"A/mod.pak": "primary"},
			remaining: map[string]string{
				"a/mod.utoc": "unrelated utoc",
				"a/mod.ucas": "unrelated ucas",
			},
		},
		{
			name:    "same stem in both folders",
			primary: "a/mod.pak",
			selected: map[string]string{
				"a/mod.pak":  "selected primary",
				"a/mod.utoc": "selected utoc",
				"a/mod.ucas": "selected ucas",
			},
			remaining: map[string]string{
				"A/mod.pak":  "unrelated primary",
				"A/mod.utoc": "unrelated utoc",
				"A/mod.ucas": "unrelated ucas",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			recycleRoot := t.TempDir()
			for _, files := range []map[string]string{test.selected, test.remaining} {
				for path, contents := range files {
					writeFile(t, filepath.Join(root, path), contents)
				}
			}
			entryID := scannedEntryID(t, root, test.primary)

			// Act
			result, err := deleteModWithRecycle(root, entryID, true, moveToDisposableRecycleBin(t, recycleRoot))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !result.Deleted || result.PreviousPrimaryPath != test.primary {
				t.Errorf("result = %#v, want deletion of %q", result, test.primary)
			}
			if got := snapshotFiles(t, root, ""); !reflect.DeepEqual(got, test.remaining) {
				t.Errorf("remaining files = %#v, want %#v", got, test.remaining)
			}
		})
	}
}
