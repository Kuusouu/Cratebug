package mutation

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/patchupdate"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Runs the patch updater against a real library with the pinned worker, the
// installed game, and real mappings. It rewrites the library it is given, so
// it runs only when CRATEBUG_PATCHUPDATE_LIBRARY names a disposable copy.
// CRATEBUG_PATCHUPDATE_PAKS names the game's Paks folder and
// CRATEBUG_PATCHUPDATE_USMAP a folder of cached mappings.
func TestUpdateModsForPatchAgainstDisposableLibrary(t *testing.T) {
	// Arrange
	library := os.Getenv("CRATEBUG_PATCHUPDATE_LIBRARY")
	paks := os.Getenv("CRATEBUG_PATCHUPDATE_PAKS")
	usmapDir := os.Getenv("CRATEBUG_PATCHUPDATE_USMAP")
	if library == "" || paks == "" || usmapDir == "" {
		t.Skip("set CRATEBUG_PATCHUPDATE_LIBRARY, _PAKS, and _USMAP to run against a disposable library")
	}
	workerPath, err := filepath.Abs(filepath.Join("..", "..", "build", uassettool.WorkerDevelopmentDirectory, uassettool.WorkerExecutableName))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(uassettool.EnvWorkerPath, workerPath)
	build, err := patchupdate.GameBuild(paks)
	if err != nil {
		t.Fatalf("GameBuild() error = %v", err)
	}
	depot := patchupdate.Depot{ListURL: "http://127.0.0.1:1/offline", CacheDir: usmapDir}
	layouts, err := patchupdate.LoadLayouts(context.Background(), depot, build, patchupdate.DefaultPreviousLayouts)
	if err != nil {
		t.Fatalf("LoadLayouts() error = %v", err)
	}
	launch := func() (ArchiveCaller, func(), error) {
		worker, err := uassettool.NewWriteWorker(nil)
		if err != nil {
			return nil, nil, err
		}
		return worker, func() { _ = worker.Close() }, nil
	}

	// Act: check the library in one worker request.
	before := bundleFileState(t, library)
	checkStart := time.Now()
	plan, err := CheckModsForPatch(context.Background(), PatchUpdateRequest{
		ModRoot:  library,
		Layouts:  layouts,
		GamePaks: paks,
	}, func(ctx context.Context, scratch string) (ArchiveCaller, func(), error) {
		worker, err := uassettool.NewPatchCheckWorker(ctx, scratch)
		if err != nil {
			return nil, nil, err
		}
		return worker, func() { _ = worker.Close() }, nil
	})
	if err != nil {
		t.Fatalf("CheckModsForPatch() error = %v", err)
	}
	checkElapsed := time.Since(checkStart)

	after := bundleFileState(t, library)
	if len(before) != len(after) {
		t.Fatalf("plan changed the live file count: %d before, %d after", len(before), len(after))
	}
	for path, state := range before {
		if after[path] != state {
			t.Errorf("plan changed live bundle file %s", path)
		}
	}
	var targets []PatchUpdateTarget
	for _, mod := range plan.Mods {
		if mod.Status == PatchUpdateFailed {
			t.Errorf("check failed for %s: %s", mod.DisplayName, mod.Detail)
		}
		if mod.Status == string(patchupdate.BundleUpdated) || mod.Status == string(patchupdate.BundlePartial) {
			targets = append(targets, PatchUpdateTarget{EntryID: mod.EntryID, DisplayName: mod.DisplayName})
		}
	}
	t.Logf("plan found %d mods to update out of %d in %s", len(targets), len(plan.Mods), checkElapsed.Round(time.Millisecond))
	if len(targets) == 0 {
		t.Skip("the disposable library has no mods an older layout affects")
	}

	start := time.Now()
	result, err := UpdateModsForPatch(PatchUpdateRequest{
		ModRoot:      library,
		Layouts:      layouts,
		GamePaks:     paks,
		OriginalsDir: filepath.Join(t.TempDir(), "originals"),
		Targets:      targets,
	}, launch, nil, nil)

	// Assert
	if err != nil {
		t.Fatalf("UpdateModsForPatch() error = %v", err)
	}
	counts := map[string]int{}
	for _, mod := range result.Mods {
		counts[mod.Status]++
		if mod.Status != string(patchupdate.BundleCurrent) {
			t.Logf("%-8s %s updated=%d blocked=%v ambiguous=%v %s", mod.Status, mod.DisplayName, mod.Updated, mod.Blocked, mod.Ambiguous, mod.Detail)
		}
	}
	t.Logf("build %d, %d selected mods in %s: %v", result.GameBuild, len(result.Mods), time.Since(start).Round(time.Second), counts)
	if counts[PatchUpdateFailed] != 0 {
		t.Errorf("%d mods failed", counts[PatchUpdateFailed])
	}
}

// Records the size and modification time of every live bundle file, which a
// plan pass must leave exactly as they were.
func bundleFileState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".pak", ".utoc", ".ucas":
			info, err := entry.Info()
			if err != nil {
				return err
			}
			state[path] = info.ModTime().UTC().String() + "|" + strconv.FormatInt(info.Size(), 10)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot bundle files: %v", err)
	}
	return state
}
