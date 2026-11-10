package patchupdate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssetOutcome says what the updater did with one asset.
type AssetOutcome string

const (
	// The asset already reads correctly in the current layout.
	AssetCurrent AssetOutcome = "current"
	// The asset was renumbered from an earlier layout and verified.
	AssetUpdated AssetOutcome = "updated"
	// The asset needs an update the updater cannot make safely.
	AssetBlocked AssetOutcome = "blocked"
	// Both an earlier layout and the current one read the asset cleanly with
	// different values, so its layout cannot be told. It is left as it is.
	AssetAmbiguous AssetOutcome = "ambiguous"
)

// AssetResult reports one asset, by its path inside the mod.
type AssetResult struct {
	Path      string       `json:"path"`
	Outcome   AssetOutcome `json:"outcome"`
	FromBuild int          `json:"fromBuild,omitempty"`
	Reason    string       `json:"reason,omitempty"`
}

// Caller is the worker request surface the updater uses.
type Caller interface {
	Call(action string, params map[string]any, result any) error
}

type assetJob struct {
	caller  Caller
	layouts *Layouts
	// Legacy .uasset path; the matching .uexp sits beside it.
	uasset  string
	scratch string
}

// The subset of get_asset_summary the updater needs. The worker can report
// this from the package header without serializing the whole asset.
type assetSummary struct {
	HasUnversionedProperties bool `json:"has_unversioned_properties"`
}

// Cheap header-only filter. Most assets in a mod are versioned (textures,
// meshes, audio) and cannot have the unversioned-layout problem this updater
// fixes, so converting them to JSON is pure cost.
func (j assetJob) hasUnversionedProperties() bool {
	var summary assetSummary
	err := j.caller.Call("get_asset_summary", map[string]any{
		"file_path":  j.uasset,
		"usmap_path": j.layouts.Current.Path,
	}, &summary)
	if err != nil {
		// A summary can fail while the asset is still readable with its real
		// layout. Fall through to the normal read so the error that matters
		// is the one the conversion reports.
		return true
	}
	return summary.HasUnversionedProperties
}

// Brings one extracted legacy asset into the current layout in place. An
// updated asset also returns its reading in the layout it was cooked with,
// which later verification compares against.
func (j assetJob) run() (AssetResult, assetRead, error) {
	if !j.hasUnversionedProperties() {
		return AssetResult{Outcome: AssetCurrent}, assetRead{}, nil
	}
	current, err := j.read(j.layouts.Current, "current")
	if err != nil {
		return AssetResult{}, assetRead{}, err
	}
	if !current.unversioned {
		return AssetResult{Outcome: AssetCurrent}, assetRead{}, nil
	}
	candidates := j.layouts.olderCandidates(current.classes)
	if len(candidates) == 0 {
		return AssetResult{Outcome: AssetCurrent}, assetRead{}, nil
	}

	for _, layout := range candidates {
		older, err := j.read(layout, fmt.Sprintf("b%d", layout.Build))
		if err != nil {
			return AssetResult{}, assetRead{}, err
		}
		if !older.clean {
			continue
		}
		if current.clean && current.sameValues(older) {
			continue
		}
		var result AssetResult
		if current.clean {
			result, err = j.resolveBothClean(current, older, layout)
		} else {
			result, err = j.update(older, layout)
		}
		return result, older, err
	}

	if current.clean {
		return AssetResult{Outcome: AssetCurrent}, assetRead{}, nil
	}
	return AssetResult{
		Outcome: AssetBlocked,
		Reason:  "no known game layout reads this asset",
	}, assetRead{}, nil
}

// Settles which reading is true when both layouts read cleanly: the one that
// writes the original bytes back exactly.
func (j assetJob) resolveBothClean(current, older assetRead, layout Layout) (AssetResult, error) {
	currentExact, err := j.writesOriginal(current, j.layouts.Current, "current-check")
	if err != nil {
		return AssetResult{}, err
	}
	olderExact, err := j.writesOriginal(older, layout, fmt.Sprintf("b%d-check", layout.Build))
	if err != nil {
		return AssetResult{}, err
	}
	switch {
	case currentExact && !olderExact:
		return AssetResult{Outcome: AssetCurrent}, nil
	case olderExact && !currentExact:
		return j.update(older, layout)
	default:
		return AssetResult{
			Outcome:   AssetAmbiguous,
			FromBuild: layout.Build,
			Reason:    fmt.Sprintf("reads cleanly in both build %d and the current layout", layout.Build),
		}, nil
	}
}

// Renumbers the older reading into the current layout, writes it, and keeps
// the result only when it reads cleanly with every value unchanged.
func (j assetJob) update(older assetRead, layout Layout) (AssetResult, error) {
	exact, err := j.writesOriginal(older, layout, fmt.Sprintf("b%d-check", layout.Build))
	if err != nil {
		return AssetResult{}, err
	}
	if !exact {
		return AssetResult{
			Outcome:   AssetBlocked,
			FromBuild: layout.Build,
			Reason:    fmt.Sprintf("build %d reads it, but cannot write it back unchanged", layout.Build),
		}, nil
	}

	remapped, _, err := RemapAssetJSON(older.document, layout.Mappings, j.layouts.Current.Mappings)
	if errors.Is(err, ErrLayoutNotPortable) {
		return AssetResult{Outcome: AssetBlocked, FromBuild: layout.Build, Reason: err.Error()}, nil
	}
	if err != nil {
		return AssetResult{}, err
	}
	dir := filepath.Join(j.scratch, "updated")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return AssetResult{}, err
	}
	jsonPath := filepath.Join(j.scratch, "remapped.json")
	if err := os.WriteFile(jsonPath, remapped, 0o600); err != nil {
		return AssetResult{}, err
	}
	output := filepath.Join(dir, filepath.Base(j.uasset))
	if err := j.caller.Call("from_json", map[string]any{
		"file_path":   jsonPath,
		"usmap_path":  j.layouts.Current.Path,
		"output_path": output,
	}, nil); err != nil {
		return AssetResult{}, fmt.Errorf("write updated asset: %w", err)
	}

	check, err := j.readFile(output, j.layouts.Current, "verify")
	if err != nil {
		return AssetResult{}, err
	}
	if !check.clean || !check.sameValues(older) {
		return AssetResult{
			Outcome:   AssetBlocked,
			FromBuild: layout.Build,
			Reason:    "the updated asset did not read back with the same values",
		}, nil
	}

	for _, ext := range []string{".uasset", ".uexp"} {
		if err := replaceFile(swapExt(output, ext), swapExt(j.uasset, ext)); err != nil {
			return AssetResult{}, fmt.Errorf("install updated asset: %w", err)
		}
	}
	return AssetResult{Outcome: AssetUpdated, FromBuild: layout.Build}, nil
}

func (j assetJob) read(layout Layout, tag string) (assetRead, error) {
	return j.readFile(j.uasset, layout, tag)
}

func (j assetJob) readFile(uasset string, layout Layout, tag string) (assetRead, error) {
	output := filepath.Join(j.scratch, tag+".json")
	if err := j.caller.Call("to_json", map[string]any{
		"file_path":   uasset,
		"usmap_path":  layout.Path,
		"output_path": output,
	}, nil); err != nil {
		return assetRead{}, fmt.Errorf("read asset with build %d layout: %w", layout.Build, err)
	}
	document, err := os.ReadFile(output)
	if err != nil {
		return assetRead{}, err
	}
	return parseAssetRead(document)
}

// Reports whether writing a reading back with its own layout reproduces the
// asset's export bytes exactly, which only the layout it was cooked with does.
func (j assetJob) writesOriginal(read assetRead, layout Layout, tag string) (bool, error) {
	jsonPath := filepath.Join(j.scratch, tag+".json")
	if err := os.WriteFile(jsonPath, read.document, 0o600); err != nil {
		return false, err
	}
	dir := filepath.Join(j.scratch, tag)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	output := filepath.Join(dir, filepath.Base(j.uasset))
	if err := j.caller.Call("from_json", map[string]any{
		"file_path":   jsonPath,
		"usmap_path":  layout.Path,
		"output_path": output,
	}, nil); err != nil {
		return false, fmt.Errorf("write asset with build %d layout: %w", layout.Build, err)
	}
	original, err := os.ReadFile(swapExt(j.uasset, ".uexp"))
	if err != nil {
		return false, err
	}
	written, err := os.ReadFile(swapExt(output, ".uexp"))
	if err != nil {
		return false, err
	}
	return bytes.Equal(original, written), nil
}

func swapExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func replaceFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
}
