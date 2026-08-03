package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanFixtureLibrary(t *testing.T) {
	root := copyFixtureLibrary(t)
	library, err := Scan(root)
	if err != nil {
		t.Fatalf("scan fixtures: %v", err)
	}
	if len(library.Entries) != 22 {
		t.Fatalf("entry count = %d, want 22", len(library.Entries))
	}

	byPrimary := make(map[string]Entry)
	for _, entry := range library.Entries {
		if entry.PrimaryPath != "" {
			byPrimary[entry.PrimaryPath] = entry
		}
	}

	for _, want := range []entryExpectation{
		{path: "enabled/Enabled_9999999_P.pak", kind: EntryMod, format: BundleFormatClassic, state: StateEnabled},
		{path: "classic/ClassicOnly_9999999_P.pak", kind: EntryMod, format: BundleFormatClassic, state: StateEnabled},
		{path: "disabled/CrateDisabled_9999999_P.pak_crateoff", kind: EntryMod, format: BundleFormatClassic, state: StateDisabled, disabledFormat: DisabledFormatCrateoff},
		{path: "disabled/BentoDisabled_9999999_P.bak_bento", kind: EntryMod, format: BundleFormatClassic, state: StateDisabled, disabledFormat: DisabledFormatBento},
		{path: "disabled/LegacyDisabled_9999999_P.pak_disabled", kind: EntryMod, format: BundleFormatClassic, state: StateDisabled, disabledFormat: DisabledFormatLegacy},
		{path: "iostore/Complete_9999999_P.pak", kind: EntryMod, format: BundleFormatIoStore, state: StateEnabled, sidecars: Sidecars{UTOC: "iostore/Complete_9999999_P.utoc", UCAS: "iostore/Complete_9999999_P.ucas"}},
		{path: "partial/MissingUcas_9999999_P.pak", kind: EntryMod, format: BundleFormatIoStore, state: StateEnabled, sidecars: Sidecars{UTOC: "partial/MissingUcas_9999999_P.utoc"}, issues: []IssueCode{IssueMissingUCAS}},
		{path: "partial/MissingUtoc_9999999_P.pak", kind: EntryMod, format: BundleFormatIoStore, state: StateEnabled, sidecars: Sidecars{UCAS: "partial/MissingUtoc_9999999_P.ucas"}, issues: []IssueCode{IssueMissingUTOC}},
		{path: "ambiguous/DualPrimary_9999999_P.pak", kind: EntryMod, format: BundleFormatClassic, state: StateEnabled, issues: []IssueCode{IssueAmbiguousPrimary}},
		{path: "ambiguous/DualPrimary_9999999_P.pak_crateoff", kind: EntryMod, format: BundleFormatClassic, state: StateDisabled, disabledFormat: DisabledFormatCrateoff, issues: []IssueCode{IssueAmbiguousPrimary}},
	} {
		assertExpectedEntry(t, byPrimary, want)
	}

	assertOrphan(t, library.Entries, Sidecars{UTOC: "orphan/OrphanUtoc.utoc"})
	assertOrphan(t, library.Entries, Sidecars{UCAS: "orphan/OrphanUcas.ucas"})

	priorityCases := map[string]struct {
		kind  PriorityKind
		value int
	}{
		"priority/!Example_9999999_P.pak":      {kind: PriorityLeadingBang, value: 0},
		"priority/Example_9999999_P.pak":       {kind: PriorityTrailingNine, value: 1},
		"priority/Example_99999999_P.pak":      {kind: PriorityTrailingNine, value: 2},
		"priority/Example_999999999_P.pak":     {kind: PriorityTrailingNine, value: 3},
		"priority/Example_P.pak":               {kind: PriorityUnrecognized, value: 0},
		"priority/ExampleNoPriority.pak":       {kind: PriorityNone, value: 0},
		"priority/Example_weirdpriority_P.pak": {kind: PriorityUnrecognized, value: 0},
	}
	for path, want := range priorityCases {
		entry, ok := byPrimary[path]
		if !ok {
			t.Errorf("missing priority fixture result for %s", path)
			continue
		}
		if entry.Priority.Kind != want.kind || entry.Priority.Value != want.value {
			t.Errorf("%s priority = %#v, want kind %q value %d", path, entry.Priority, want.kind, want.value)
		}
	}
}

func TestScanRejectsInvalidRoots(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "missing"), file} {
		if _, err := Scan(path); err == nil {
			t.Errorf("Scan(%q) succeeded, want error", path)
		}
	}
}

func TestScanEmptyRoot(t *testing.T) {
	library, err := Scan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if library.Entries == nil {
		t.Fatal("entries = nil, want an empty slice")
	}

	if len(library.Entries) != 0 {
		t.Fatalf("entry count = %d, want 0", len(library.Entries))
	}
}

func TestScanIgnoresUnrelatedFilesAndPreservesFolders(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "A", "B"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "A", "B", "Nested_9999999_P.pak")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	library, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(library.Entries) != 1 || library.Entries[0].RelativeFolder != "A/B" {
		t.Fatalf("entries = %#v, want one entry in A/B", library.Entries)
	}
}

func TestScanGroupsMixedCaseNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Example.pak", "example.utoc"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	library, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(library.Entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(library.Entries))
	}
	entry := library.Entries[0]
	if entry.PrimaryPath != "Example.pak" || entry.Sidecars.UTOC != "example.utoc" || entry.BundleFormat != BundleFormatIoStore || !hasIssue(entry.Issues, IssueMissingUCAS) {
		t.Fatalf("mixed-case entry = %#v, want one incomplete IoStore entry with original path casing", entry)
	}
}

func TestScanPreservesSimultaneousIssues(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Example.pak", "Example.pak_crateoff", "Example.utoc"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	library, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(library.Entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(library.Entries))
	}
	for _, entry := range library.Entries {
		if entry.BundleFormat != BundleFormatIoStore || !hasIssue(entry.Issues, IssueMissingUCAS) || !hasIssue(entry.Issues, IssueAmbiguousPrimary) {
			t.Fatalf("entry %#v does not preserve both incomplete and ambiguous issues", entry)
		}
	}
}

func TestScanDoesNotModifyFiles(t *testing.T) {
	root := copyFixtureLibrary(t)
	before := snapshotFiles(t, root)
	if _, err := Scan(root); err != nil {
		t.Fatal(err)
	}
	after := snapshotFiles(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Scan changed fixture files")
	}
}

func TestScanIsRepeatableAndReflectsFilesystemChanges(t *testing.T) {
	root := copyFixtureLibrary(t)
	first, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated scans of unchanged files differ")
	}
	newPath := filepath.Join(root, "new", "Added_9999999_P.pak")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	third, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Entries) != len(first.Entries)+1 {
		t.Fatalf("after adding a file, entry count = %d, want %d", len(third.Entries), len(first.Entries)+1)
	}
}

func TestScanOrderingIsDeterministic(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Z_9999999_P.pak", "A_9999999_P.pak", "M_9999999_P.pak"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	library, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(library.Entries))
	for _, entry := range library.Entries {
		paths = append(paths, entry.PrimaryPath)
	}
	want := []string{"A_9999999_P.pak", "M_9999999_P.pak", "Z_9999999_P.pak"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

type entryExpectation struct {
	path           string
	kind           EntryKind
	format         BundleFormat
	state          State
	disabledFormat DisabledFormat
	sidecars       Sidecars
	issues         []IssueCode
}

func assertExpectedEntry(t *testing.T, entries map[string]Entry, want entryExpectation) {
	t.Helper()
	entry, ok := entries[want.path]
	if !ok {
		t.Errorf("missing entry for %s", want.path)
		return
	}
	if entry.Kind != want.kind || entry.BundleFormat != want.format || entry.State != want.state || entry.DisabledFormat != want.disabledFormat || entry.Sidecars != want.sidecars || !reflect.DeepEqual(issueCodes(entry.Issues), want.issues) {
		t.Errorf("entry %s = %#v, want %#v", want.path, entry, want)
	}
}

func assertOrphan(t *testing.T, entries []Entry, sidecars Sidecars) {
	t.Helper()
	for _, entry := range entries {
		if entry.Kind == EntryOrphanedSidecar && entry.Sidecars == sidecars && reflect.DeepEqual(issueCodes(entry.Issues), []IssueCode{IssueOrphanedSidecar}) {
			return
		}
	}
	t.Errorf("missing orphan entry with sidecars %#v", sidecars)
}

func hasIssue(issues []Issue, code IssueCode) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func issueCodes(issues []Issue) []IssueCode {
	if len(issues) == 0 {
		return nil
	}
	codes := make([]IssueCode, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}

func copyFixtureLibrary(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "library")
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o600)
	})
	if err != nil {
		t.Fatalf("copy fixtures: %v", err)
	}
	return destination
}

type fileSnapshot struct {
	contents string
	mode     fs.FileMode
	modTime  int64
}

func snapshotFiles(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()
	snapshot := make(map[string]fileSnapshot)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = fileSnapshot{contents: string(contents), mode: info.Mode(), modTime: info.ModTime().UnixNano()}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot files: %v", err)
	}
	return snapshot
}
