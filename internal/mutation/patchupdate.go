package mutation

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/patchupdate"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Status reported for a mod whose update failed outright.
const PatchUpdateFailed = "failed"

// PatchUpdateTarget names a mod the user selected for an update.
type PatchUpdateTarget struct {
	EntryID     string `json:"entryID"`
	DisplayName string `json:"displayName"`
}

// PatchUpdateRequest names the library, the layouts to carry mods between,
// and where the originals of rewritten bundles are kept. An empty Targets
// selects every complete IoStore mod.
type PatchUpdateRequest struct {
	ModRoot  string
	Layouts  *patchupdate.Layouts
	GamePaks string
	// Each rewritten bundle's original files are copied here first, under
	// their library-relative paths. An earlier copy is never overwritten.
	OriginalsDir string
	Targets      []PatchUpdateTarget
}

// PatchUpdateAsset reports one non-current asset in a mod.
type PatchUpdateAsset struct {
	Path      string `json:"path"`
	Outcome   string `json:"outcome"`
	FromBuild int    `json:"fromBuild,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// PatchUpdateMod reports one mod and the assets that need attention.
type PatchUpdateMod struct {
	EntryID        string             `json:"entryID"`
	DisplayName    string             `json:"displayName"`
	RelativeFolder string             `json:"relativeFolder"`
	Enabled        bool               `json:"enabled"`
	Status         string             `json:"status"`
	Detail         string             `json:"detail,omitempty"`
	Updated        int                `json:"updated"`
	FromBuilds     []int              `json:"fromBuilds,omitempty"`
	Blocked        []string           `json:"blocked,omitempty"`
	Ambiguous      []string           `json:"ambiguous,omitempty"`
	Assets         []PatchUpdateAsset `json:"assets,omitempty"`
}

// PatchUpdatePlan reports what a check found, without changing any file.
type PatchUpdatePlan struct {
	GameBuild int              `json:"gameBuild"`
	GamePaks  string           `json:"gamePaks"`
	Mods      []PatchUpdateMod `json:"mods"`
}

// PatchUpdateResult reports every mod an apply processed.
type PatchUpdateResult struct {
	GameBuild    int              `json:"gameBuild"`
	OriginalsDir string           `json:"originalsDir"`
	Mods         []PatchUpdateMod `json:"mods"`
	Cancelled    bool             `json:"cancelled"`
}

// PatchUpdateProgress identifies the mod being checked or updated.
type PatchUpdateProgress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	EntryID     string `json:"entryID"`
	DisplayName string `json:"displayName"`
	Phase       string `json:"phase"`
}

// Rewrites the selected IoStore mods whose assets an earlier game layout
// cooked. Each rewrite is verified on a private copy before the live files
// change, and the live originals are copied to OriginalsDir first.
func UpdateModsForPatch(
	request PatchUpdateRequest,
	launch ArchiveCallerFactory,
	progress func(PatchUpdateProgress),
	cancel <-chan struct{},
) (PatchUpdateResult, error) {
	if launch == nil || request.Layouts == nil {
		return PatchUpdateResult{}, fmt.Errorf("patch update needs a worker factory and layouts")
	}
	root, targets, missing, err := scanPatchTargets(request.ModRoot, request.Targets)
	if err != nil {
		return PatchUpdateResult{}, err
	}

	result := PatchUpdateResult{
		GameBuild:    request.Layouts.Current.Build,
		OriginalsDir: request.OriginalsDir,
		Mods:         missing,
	}
	if len(targets) == 0 {
		return result, nil
	}

	size := uassettool.DefaultWorkerPoolSizeForLibrary(len(targets))
	size = max(1, min(size, len(targets)))
	callers := make([]ArchiveCaller, 0, size)
	var cleanups []func()
	defer func() {
		for _, cleanup := range cleanups {
			if cleanup != nil {
				cleanup()
			}
		}
	}()
	for range size {
		caller, cleanup, err := launch()
		if err != nil {
			return PatchUpdateResult{}, fmt.Errorf("start patch update worker: %w", err)
		}
		callers = append(callers, caller)
		cleanups = append(cleanups, cleanup)
	}

	jobs := make(chan discovery.Entry)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var started atomic.Int32
	for _, caller := range callers {
		wg.Add(1)
		go func(caller ArchiveCaller) {
			defer wg.Done()
			for entry := range jobs {
				if progress != nil {
					progress(PatchUpdateProgress{
						Current:     int(started.Add(1)),
						Total:       len(targets),
						EntryID:     entry.ID,
						DisplayName: entry.DisplayName,
						Phase:       "updating",
					})
				}
				mod := updateModForPatch(root, entry, request, caller)
				mu.Lock()
				result.Mods = append(result.Mods, mod)
				mu.Unlock()
			}
		}(caller)
	}

	sent := 0
	for _, entry := range targets {
		if cancelled(cancel) {
			break
		}
		jobs <- entry
		sent++
	}
	close(jobs)
	wg.Wait()

	sortPatchMods(result.Mods)
	if sent < len(targets) {
		result.Cancelled = true
	}
	return result, nil
}

func sortPatchMods(mods []PatchUpdateMod) {
	sort.Slice(mods, func(i, j int) bool {
		return strings.ToLower(mods[i].DisplayName) < strings.ToLower(mods[j].DisplayName)
	})
}

func scanPatchTargets(modRoot string, requested []PatchUpdateTarget) (string, []discovery.Entry, []PatchUpdateMod, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return "", nil, nil, fmt.Errorf("scan mod library for patch updater: %w", err)
	}
	root, err := filepath.Abs(library.Root)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolve mod root: %w", err)
	}
	targets, missing := selectPatchTargets(library.Entries, requested)
	return root, targets, missing, nil
}

// selectPatchTargets keeps the requested entries that are complete IoStore
// bundles and reports the rest as failed, so a mod moved or deleted while the
// dialog was open still has an answer in the result.
func selectPatchTargets(entries []discovery.Entry, requested []PatchUpdateTarget) ([]discovery.Entry, []PatchUpdateMod) {
	if len(requested) == 0 {
		var targets []discovery.Entry
		for _, entry := range entries {
			if validateEncryptableEntry(entry) == nil {
				targets = append(targets, entry)
			}
		}
		return targets, nil
	}

	byID := make(map[string]discovery.Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	var targets []discovery.Entry
	var failed []PatchUpdateMod
	for _, target := range requested {
		name := target.DisplayName
		if name == "" {
			name = target.EntryID
		}
		entry, ok := byID[target.EntryID]
		if !ok {
			failed = append(failed, PatchUpdateMod{
				EntryID:     target.EntryID,
				DisplayName: name,
				Status:      PatchUpdateFailed,
				Detail:      "This mod is no longer in the library.",
			})
			continue
		}
		if err := validateEncryptableEntry(entry); err != nil {
			failed = append(failed, PatchUpdateMod{
				EntryID:        entry.ID,
				DisplayName:    entry.DisplayName,
				RelativeFolder: entry.RelativeFolder,
				Enabled:        entry.State == discovery.StateEnabled,
				Status:         PatchUpdateFailed,
				Detail:         "This mod is not a complete IoStore bundle.",
			})
			continue
		}
		targets = append(targets, entry)
	}
	return targets, failed
}

func patchMod(entry discovery.Entry) PatchUpdateMod {
	return PatchUpdateMod{
		EntryID:        entry.ID,
		DisplayName:    entry.DisplayName,
		RelativeFolder: entry.RelativeFolder,
		Enabled:        entry.State == discovery.StateEnabled,
	}
}

func updateModForPatch(root string, entry discovery.Entry, request PatchUpdateRequest, caller ArchiveCaller) PatchUpdateMod {
	mod := patchMod(entry)
	bundle, err := rewriteBundleForPatch(root, entry, request, caller)
	if err != nil {
		mod.Status = PatchUpdateFailed
		mod.Detail = err.Error()
		return mod
	}
	return reportPatchBundle(mod, bundle)
}

func reportPatchBundle(mod PatchUpdateMod, bundle patchupdate.BundleResult) PatchUpdateMod {
	mod.Status = string(bundle.Status)
	mod.Detail = bundle.Detail

	builds := map[int]bool{}
	for _, asset := range bundle.Assets {
		switch asset.Outcome {
		case patchupdate.AssetUpdated:
			mod.Updated++
		case patchupdate.AssetBlocked:
			mod.Blocked = append(mod.Blocked, asset.Path+": "+asset.Reason)
		case patchupdate.AssetAmbiguous:
			mod.Ambiguous = append(mod.Ambiguous, asset.Path)
		}
		if asset.FromBuild > 0 {
			builds[asset.FromBuild] = true
		}
		if asset.Outcome != patchupdate.AssetCurrent {
			mod.Assets = append(mod.Assets, PatchUpdateAsset{
				Path:      asset.Path,
				Outcome:   string(asset.Outcome),
				FromBuild: asset.FromBuild,
				Reason:    asset.Reason,
			})
		}
	}
	for build := range builds {
		mod.FromBuilds = append(mod.FromBuilds, build)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(mod.FromBuilds)))
	return mod
}

func patchBundleFiles(root string, entry discovery.Entry) (map[string]string, error) {
	live := map[string]string{
		".pak":  filepath.Join(root, filepath.FromSlash(entry.PrimaryPath)),
		".utoc": filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC)),
		".ucas": filepath.Join(root, filepath.FromSlash(entry.Sidecars.UCAS)),
	}
	for _, path := range live {
		if !pathWithinRoot(root, path) {
			return nil, fmt.Errorf("bundle paths escape the mod root")
		}
		if err := requireRegularFile(path, "live bundle file"); err != nil {
			return nil, err
		}
	}
	return live, nil
}

func rewriteBundleForPatch(root string, entry discovery.Entry, request PatchUpdateRequest, caller ArchiveCaller) (patchupdate.BundleResult, error) {
	live, err := patchBundleFiles(root, entry)
	if err != nil {
		return patchupdate.BundleResult{}, err
	}
	workDir, err := os.MkdirTemp("", "cratebug-patch-*")
	if err != nil {
		return patchupdate.BundleResult{}, fmt.Errorf("create patch update work directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(workDir) // Best-effort removal of disposable staging; preserve the operation result.
	}()

	// A plain .pak name lets the worker find the sibling .utoc even when the
	// live primary is disabled (.pak_crateoff). Every action in the update is
	// read-only against this staged copy, so a hard link is safe and avoids
	// copying entire multi-gigabyte .ucas files to temp. Cross-volume staging
	// falls back to a normal copy.
	staged := map[string]string{}
	for ext, path := range live {
		staged[ext] = filepath.Join(workDir, "bundle"+ext)
		if err := stageBundleFile(path, staged[ext]); err != nil {
			return patchupdate.BundleResult{}, fmt.Errorf("stage bundle file: %w", err)
		}
	}

	encrypted, err := uassettool.IsIoStoreEncrypted(caller, staged[".utoc"])
	if err != nil {
		return patchupdate.BundleResult{}, fmt.Errorf("check encryption: %w", err)
	}
	listing, err := uassettool.ListIoStoreFiles(caller, staged[".utoc"], uassettool.MarvelRivalsAESKey)
	if err != nil {
		return patchupdate.BundleResult{}, fmt.Errorf("list bundle: %w", err)
	}

	bundle, err := patchupdate.UpdateBundle(caller, request.Layouts, staged[".pak"], staged[".utoc"], filepath.Join(workDir, "job"), patchupdate.BundleOptions{
		GamePaks:      request.GamePaks,
		Encrypt:       encrypted || modtype.RequiresIoStoreEncryption(listing),
		ContainerName: patchContainerName(entry),
	})
	if err != nil {
		return patchupdate.BundleResult{}, err
	}
	if bundle.PAK == "" {
		return bundle, nil
	}

	if err := keepOriginals(root, request.OriginalsDir, live); err != nil {
		return patchupdate.BundleResult{}, fmt.Errorf("keep original files: %w", err)
	}
	replacements := map[string]string{
		live[".pak"]:  bundle.PAK,
		live[".utoc"]: bundle.UTOC,
		live[".ucas"]: bundle.UCAS,
	}
	if err := replaceBundleFiles(root, replacements); err != nil {
		return patchupdate.BundleResult{}, err
	}
	return bundle, nil
}

// The container ID derives from this name, so it folds in the mod's path:
// two mods with the same file name in different folders must not share one.
func patchContainerName(entry discovery.Entry) string {
	stem := filepath.Base(filepath.FromSlash(entry.PrimaryPath))
	stem = strings.TrimSuffix(stem, filepath.Ext(stem))
	checksum := crc32.ChecksumIEEE([]byte(strings.ToLower(entry.PrimaryPath)))
	return fmt.Sprintf("%s_cb%08x", stem, checksum)
}

// stageBundleFile hard-links the live bundle into the work directory when
// both paths share a volume. The update only reads the staged file, so the
// link cannot write back to the library. A cross-volume library falls back to
// the old copy.
func stageBundleFile(source, destination string) error {
	if err := os.Link(source, destination); err == nil {
		return nil
	}
	return copyRegularFile(source, destination)
}

func keepOriginals(root, originalsDir string, live map[string]string) error {
	if originalsDir == "" {
		return fmt.Errorf("no folder for original files")
	}
	for _, path := range live {
		rel, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("bundle path escapes the mod root")
		}
		destination := filepath.Join(originalsDir, rel)
		if _, err := os.Stat(destination); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		if err := copyRegularFile(path, destination); err != nil {
			return err
		}
	}
	return nil
}
