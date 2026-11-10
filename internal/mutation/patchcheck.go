package mutation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/patchupdate"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Checks the selected bundles in one worker request and joins the verdicts
// to the mod entries the library scanner found.
func CheckModsForPatch(
	ctx context.Context,
	request PatchUpdateRequest,
	launch func(context.Context, string) (ArchiveCaller, func(), error),
) (PatchUpdatePlan, error) {
	if launch == nil || request.Layouts == nil {
		return PatchUpdatePlan{}, fmt.Errorf("patch check needs a worker factory and layouts")
	}
	if err := ctx.Err(); err != nil {
		return PatchUpdatePlan{}, err
	}
	root, targets, missing, err := scanPatchTargets(request.ModRoot, request.Targets)
	if err != nil {
		return PatchUpdatePlan{}, err
	}
	plan := PatchUpdatePlan{
		GameBuild: request.Layouts.Current.Build,
		GamePaks:  request.GamePaks,
		Mods:      missing,
	}
	if len(targets) == 0 {
		return plan, nil
	}

	scratch, err := os.MkdirTemp("", "cratebug-patch-check-*")
	if err != nil {
		return PatchUpdatePlan{}, fmt.Errorf("create patch check work directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(scratch) // Best-effort removal of private staging after the worker exits.
	}()

	bundles := make([]uassettool.PatchCheckBundle, 0, len(targets))
	indices := make([]int, 0, len(targets))
	for i, entry := range targets {
		if err := ctx.Err(); err != nil {
			return PatchUpdatePlan{}, err
		}
		mod := patchMod(entry)
		live, err := patchBundleFiles(root, entry)
		var bundle uassettool.PatchCheckBundle
		if err == nil {
			bundle, err = preparePatchCheckBundle(live, filepath.Join(scratch, strconv.Itoa(i)))
		}
		if err != nil {
			mod.Status = PatchUpdateFailed
			mod.Detail = err.Error()
		} else {
			bundles = append(bundles, bundle)
			indices = append(indices, len(plan.Mods))
		}
		plan.Mods = append(plan.Mods, mod)
	}

	if len(bundles) > 0 {
		caller, cleanup, err := launch(ctx, scratch)
		if err != nil {
			return PatchUpdatePlan{}, fmt.Errorf("start patch check worker: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}

		layouts := make([]uassettool.PatchCheckLayout, 0, 1+len(request.Layouts.Previous))
		for _, layout := range append([]patchupdate.Layout{request.Layouts.Current}, request.Layouts.Previous...) {
			layouts = append(layouts, uassettool.PatchCheckLayout{Build: layout.Build, UsmapPath: layout.Path})
		}
		verdicts, err := uassettool.CheckIoStorePatch(caller, request.GamePaks, bundles, layouts)
		if cancelErr := ctx.Err(); cancelErr != nil {
			return PatchUpdatePlan{}, cancelErr
		}
		if err != nil {
			return PatchUpdatePlan{}, fmt.Errorf("check mods against the game patch: %w", err)
		}
		for i, verdict := range verdicts {
			bundle := patchupdate.BundleResult{Status: patchupdate.BundleStatus(verdict.Status), Detail: verdict.Detail}
			for _, asset := range verdict.Assets {
				bundle.Assets = append(bundle.Assets, patchupdate.AssetResult{
					Path: asset.Path, Outcome: patchupdate.AssetOutcome(asset.Outcome), FromBuild: asset.FromBuild, Reason: asset.Reason,
				})
			}
			index := indices[i]
			plan.Mods[index] = reportPatchBundle(plan.Mods[index], bundle)
		}
	}

	sortPatchMods(plan.Mods)
	return plan, nil
}

func preparePatchCheckBundle(live map[string]string, scratch string) (uassettool.PatchCheckBundle, error) {
	bundle := uassettool.PatchCheckBundle{UTOCPath: live[".utoc"], PAKPath: live[".pak"]}
	if strings.TrimSuffix(bundle.UTOCPath, filepath.Ext(bundle.UTOCPath))+".ucas" == live[".ucas"] {
		return bundle, nil
	}

	// Discovery accepts mixed-case stems and extensions. The worker opens the
	// UCAS by name, so stage matching sidecar names when the originals differ.
	if err := os.Mkdir(scratch, 0o700); err != nil {
		return uassettool.PatchCheckBundle{}, fmt.Errorf("create sidecar staging directory: %w", err)
	}
	for _, ext := range []string{".utoc", ".ucas"} {
		if err := stageBundleFile(live[ext], filepath.Join(scratch, "bundle"+ext)); err != nil {
			return uassettool.PatchCheckBundle{}, fmt.Errorf("stage bundle sidecar: %w", err)
		}
	}
	bundle.UTOCPath = filepath.Join(scratch, "bundle.utoc")
	return bundle, nil
}
