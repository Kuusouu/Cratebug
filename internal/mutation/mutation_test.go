package mutation

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

type transitionTest struct {
	name        string
	source      string
	enabled     bool
	destination string
	state       discovery.State
}

type rejectedTransitionTest struct {
	name    string
	files   []string
	primary string
	enabled bool
}

// Covers all supported filename transitions with isolated filesystem fixtures.
func TestSetEnabledTransitionsPrimaryWithoutChangingSidecars(t *testing.T) {
	for _, test := range []transitionTest{
		{name: "disable", source: "Example_9999999_P.pak", destination: "Example_9999999_P.pak_crateoff", state: discovery.StateDisabled},
		{name: "disable nested", source: "nested/Example_9999999_P.pak", destination: "nested/Example_9999999_P.pak_crateoff", state: discovery.StateDisabled},
		{name: "enable cratebug", source: "Example_9999999_P.pak_crateoff", enabled: true, destination: "Example_9999999_P.pak", state: discovery.StateEnabled},
		{name: "enable BentoMod", source: "Example_9999999_P.bak_bento", enabled: true, destination: "Example_9999999_P.pak", state: discovery.StateEnabled},
		{name: "enable legacy", source: "Example_9999999_P.pak_disabled", enabled: true, destination: "Example_9999999_P.pak", state: discovery.StateEnabled},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			writeFile(t, filepath.Join(root, test.source), "primary")
			sidecarFolder := filepath.Dir(test.source)
			writeFile(t, filepath.Join(root, sidecarFolder, "Example_9999999_P.utoc"), "utoc")
			writeFile(t, filepath.Join(root, sidecarFolder, "Example_9999999_P.ucas"), "ucas")
			beforeSidecars := snapshotFiles(t, root, ".utoc", ".ucas")

			// Act
			result, err := SetEnabled(root, test.source, test.enabled)
			if err != nil {
				t.Fatal(err)
			}
			// Assert
			if result.PrimaryPath != test.destination {
				t.Errorf("PrimaryPath = %q, want %q", result.PrimaryPath, test.destination)
			}
			if result.State != test.state {
				t.Errorf("State = %q, want %q", result.State, test.state)
			}
			if _, err := os.Lstat(filepath.Join(root, test.source)); !os.IsNotExist(err) {
				t.Errorf("source still exists or could not be checked: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, test.destination)); err != nil {
				t.Errorf("destination is missing: %v", err)
			}
			if afterSidecars := snapshotFiles(t, root, ".utoc", ".ucas"); !reflect.DeepEqual(beforeSidecars, afterSidecars) {
				t.Errorf("sidecars changed\nbefore: %#v\nafter: %#v", beforeSidecars, afterSidecars)
			}
		})
	}
}

// Ensures every rejected plan leaves the complete fixture unchanged.
func TestSetEnabledRejectsUnsafeOrInvalidTransitionsWithoutChanges(t *testing.T) {
	for _, test := range []rejectedTransitionTest{
		{name: "ambiguous primary", files: []string{"Example.pak", "Example.pak_crateoff"}, primary: "Example.pak_crateoff", enabled: true},
		{name: "missing scanner entry", files: []string{"Example.pak"}, primary: "missing.pak"},
		{name: "already enabled", files: []string{"Example.pak"}, primary: "Example.pak", enabled: true},
		{name: "path traversal", files: []string{"Example.pak"}, primary: "../Example.pak"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, filepath.Join(root, file), file)
			}
			before := snapshotFiles(t, root, "")

			// Act
			if _, err := SetEnabled(root, test.primary, test.enabled); err == nil {
				t.Fatal("SetEnabled succeeded, want error")
			}

			// Assert
			if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
				t.Errorf("rejected operation changed files\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

// Exercises collision handling without a second primary, which the scanner rejects as ambiguous first.
func TestSetEnabledRejectsDestinationCollisionWithoutChanges(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Example.pak"), "primary")
	if err := os.Mkdir(filepath.Join(root, "Example.pak_crateoff"), 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, root, "")

	// Act
	if _, err := SetEnabled(root, "Example.pak", false); err == nil {
		t.Fatal("SetEnabled succeeded, want destination collision error")
	}

	// Assert
	if after := snapshotFiles(t, root, ""); !reflect.DeepEqual(before, after) {
		t.Errorf("collision changed files\nbefore: %#v\nafter: %#v", before, after)
	}
}

// Confirms a destination created after planning is never overwritten by the native rename.
func TestPlanApplyRejectsDestinationCreatedAfterPlanning(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Example.pak"), "source")
	library, err := discovery.Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := buildPlan(root, library.Entries[0], false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "Example.pak_crateoff"), "destination")

	// Act
	if err := plan.apply(); err == nil {
		t.Fatal("plan.apply succeeded, want destination collision error")
	}

	// Assert
	assertFileContents(t, filepath.Join(root, "Example.pak"), "source")
	assertFileContents(t, filepath.Join(root, "Example.pak_crateoff"), "destination")
}

// Verifies the operation only accepts entries that the current scanner produced.
func TestSetEnabledUsesCurrentScannerState(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Example.pak"), "primary")

	// Act
	if _, err := SetEnabled(root, "Example.pak", false); err != nil {
		t.Fatal(err)
	}
	if _, err := SetEnabled(root, "Example.pak", false); err == nil {
		t.Fatal("stale primary path succeeded, want error")
	}
	if _, err := SetEnabled(root, "Example.pak_crateoff", true); err != nil {
		t.Fatal(err)
	}

	// Assert
	if _, err := os.Lstat(filepath.Join(root, "Example.pak")); err != nil {
		t.Errorf("enabled primary is missing: %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshotFiles(t *testing.T, root string, suffixes ...string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !matchesSuffix(entry.Name(), suffixes) {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != want {
		t.Errorf("contents of %q = %q, want %q", path, got, want)
	}
}

func matchesSuffix(name string, suffixes []string) bool {
	if len(suffixes) == 0 || suffixes[0] == "" {
		return true
	}
	for _, suffix := range suffixes {
		if filepath.Ext(name) == suffix {
			return true
		}
	}
	return false
}
