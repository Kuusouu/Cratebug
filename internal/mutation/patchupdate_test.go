package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/patchupdate"
)

func TestPatchContainerNameDiffersForSameFileNameInOtherFolders(t *testing.T) {
	// Arrange
	wedding := discovery.Entry{PrimaryPath: "Rogue/Wedding/UILopunnyRogue_9999999_P.pak"}
	plain := discovery.Entry{PrimaryPath: "Rogue/Default/UILopunnyRogue_9999999_P.pak"}
	disabled := discovery.Entry{PrimaryPath: "Rogue/Default/UILopunnyRogue_9999999_P.pak_crateoff"}

	// Act
	first, second, third := patchContainerName(wedding), patchContainerName(plain), patchContainerName(disabled)

	// Assert
	if first == second {
		t.Errorf("both folders got container name %q", first)
	}
	if !strings.HasPrefix(third, "UILopunnyRogue_9999999_P_cb") {
		t.Errorf("disabled entry name = %q, want the stem without .pak_crateoff", third)
	}
}

func TestKeepOriginalsCopiesOnceAndNeverOverwritesAnEarlierOriginal(t *testing.T) {
	// Arrange
	root := t.TempDir()
	originals := t.TempDir()
	live := filepath.Join(root, "Hero", "Mod_9999999_P.utoc")
	if err := os.MkdirAll(filepath.Dir(live), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{".utoc": live}

	// Act
	firstErr := keepOriginals(root, originals, files)
	if err := os.WriteFile(live, []byte("rewritten"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondErr := keepOriginals(root, originals, files)

	// Assert
	if firstErr != nil || secondErr != nil {
		t.Fatalf("keepOriginals() errors = %v, %v", firstErr, secondErr)
	}
	kept, err := os.ReadFile(filepath.Join(originals, "Hero", "Mod_9999999_P.utoc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "first" {
		t.Errorf("kept original = %q, want the first copy", kept)
	}
}

func TestUpdateModsForPatchRequiresLayoutsAndAWorker(t *testing.T) {
	// Arrange
	request := PatchUpdateRequest{ModRoot: t.TempDir()}

	// Act
	_, err := UpdateModsForPatch(request, nil, nil, nil)

	// Assert
	if err == nil {
		t.Error("UpdateModsForPatch() without layouts or a worker succeeded")
	}
}

func TestSelectPatchTargetsReportsMissingAndIncompleteMods(t *testing.T) {
	// Arrange
	complete := discovery.Entry{
		ID:           "complete",
		DisplayName:  "Complete",
		Kind:         discovery.EntryMod,
		PrimaryPath:  "Complete_9999999_P.pak",
		BundleFormat: discovery.BundleFormatIoStore,
		Sidecars:     discovery.Sidecars{UTOC: "Complete_9999999_P.utoc", UCAS: "Complete_9999999_P.ucas"},
	}
	classic := discovery.Entry{
		ID:           "classic",
		DisplayName:  "Classic",
		Kind:         discovery.EntryMod,
		PrimaryPath:  "Classic_9999999_P.pak",
		BundleFormat: discovery.BundleFormatClassic,
	}

	// Act
	targets, failed := selectPatchTargets(
		[]discovery.Entry{complete, classic},
		[]PatchUpdateTarget{
			{EntryID: "complete", DisplayName: "Complete"},
			{EntryID: "classic", DisplayName: "Classic"},
			{EntryID: "gone", DisplayName: "Gone"},
		},
	)

	// Assert
	if len(targets) != 1 || targets[0].ID != "complete" {
		t.Fatalf("targets = %+v, want only the complete IoStore mod", targets)
	}
	if len(failed) != 2 {
		t.Fatalf("failed = %+v, want the classic and missing mods", failed)
	}
	if failed[0].Status != PatchUpdateFailed || failed[0].DisplayName != "Classic" {
		t.Errorf("classic failure = %+v", failed[0])
	}
	if failed[1].Status != PatchUpdateFailed || failed[1].DisplayName != "Gone" {
		t.Errorf("missing failure = %+v", failed[1])
	}
}

func TestUpdateModsForPatchReportsMissingTargetsWithoutStartingAWorker(t *testing.T) {
	// Arrange
	request := PatchUpdateRequest{
		ModRoot: t.TempDir(),
		Layouts: patchupdate.NewLayouts(patchupdate.Layout{Build: 4008108}, nil),
		Targets: []PatchUpdateTarget{{EntryID: "gone", DisplayName: "Gone"}},
	}
	launch := func() (ArchiveCaller, func(), error) {
		t.Fatal("a missing target started a worker")
		return nil, nil, nil
	}

	// Act
	result, err := UpdateModsForPatch(request, launch, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("UpdateModsForPatch() error = %v", err)
	}
	if len(result.Mods) != 1 || result.Mods[0].Status != PatchUpdateFailed {
		t.Errorf("result.Mods = %+v, want one failed target", result.Mods)
	}
	if !strings.Contains(result.Mods[0].Detail, "no longer in the library") {
		t.Errorf("detail = %q, want the missing-mod message", result.Mods[0].Detail)
	}
}
