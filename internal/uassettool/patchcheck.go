package uassettool

import "fmt"

// Paths to the files the worker needs for one IoStore bundle. The UCAS must
// be beside the UTOC with the same stem and a .ucas suffix.
type PatchCheckBundle struct {
	UTOCPath string `json:"file_path"`
	PAKPath  string `json:"pak_path"`
}

// Identifies the mappings file for one game build.
type PatchCheckLayout struct {
	Build     int    `json:"build"`
	UsmapPath string `json:"usmap_path"`
}

// Reports a bundle's compatibility and the assets that need attention.
type PatchCheckVerdict struct {
	UTOCPath string            `json:"file_path"`
	Status   string            `json:"status"`
	Detail   string            `json:"detail"`
	Assets   []PatchCheckAsset `json:"assets"`
}

// Reports a non-current asset in a compatibility check.
type PatchCheckAsset struct {
	Path      string `json:"path"`
	Outcome   string `json:"outcome"`
	FromBuild int    `json:"from_build"`
	Reason    string `json:"reason"`
}

// Checks all bundles in one read-only worker request. Layouts start with the
// current build, followed by older builds in descending order.
func CheckIoStorePatch(c caller, gamePaks string, bundles []PatchCheckBundle, layouts []PatchCheckLayout) ([]PatchCheckVerdict, error) {
	if len(bundles) == 0 || len(layouts) == 0 {
		return nil, fmt.Errorf("uassettool: check_iostore_patch needs bundles and layouts")
	}
	var result struct {
		GameBuild int                 `json:"game_build"`
		Bundles   []PatchCheckVerdict `json:"bundles"`
	}
	if err := c.Call("check_iostore_patch", map[string]any{
		"game_paks":     gamePaks,
		"aes_key":       MarvelRivalsAESKey,
		"patch_bundles": bundles,
		"patch_layouts": layouts,
	}, &result); err != nil {
		return nil, err
	}
	if result.GameBuild != layouts[0].Build || len(result.Bundles) != len(bundles) {
		return nil, fmt.Errorf("%w: patch check returned an unexpected build or bundle count", ErrMalformedResponse)
	}
	for i, bundle := range result.Bundles {
		if bundle.UTOCPath != bundles[i].UTOCPath || bundle.Status == "" {
			return nil, fmt.Errorf("%w: patch check returned an unexpected verdict for bundle %d", ErrMalformedResponse, i+1)
		}
	}
	return result.Bundles, nil
}
