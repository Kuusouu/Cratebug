package mutation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/patchupdate"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

type patchCheckCaller func(action string, params map[string]any, result any) error

func (c patchCheckCaller) Call(action string, params map[string]any, result any) error {
	return c(action, params, result)
}

func TestCheckModsForPatchBatchesAndMapsBundleVerdicts(t *testing.T) {
	// Arrange
	root := t.TempDir()
	for _, name := range []string{"A.pak", "A.utoc", "A.ucas", "B.pak_crateoff", "B.utoc", "B.UCAS", "Classic.pak"} {
		writeFile(t, filepath.Join(root, name), name)
	}
	layouts := patchupdate.NewLayouts(patchupdate.Layout{Build: 200, Path: "new.usmap"}, []patchupdate.Layout{
		{Build: 100, Path: "old.usmap"},
	})
	var scratch string
	var starts, calls int
	launch := func(_ context.Context, tempDir string) (ArchiveCaller, func(), error) {
		starts++
		scratch = tempDir
		return patchCheckCaller(func(action string, params map[string]any, result any) error {
			calls++
			if action != "check_iostore_patch" {
				t.Fatalf("worker action = %s, want one patch check", action)
			}
			bundles, ok := params["patch_bundles"].([]uassettool.PatchCheckBundle)
			if !ok || len(bundles) != 2 {
				t.Fatalf("patch_bundles = %v, want two IoStore bundles", params["patch_bundles"])
			}
			if bundles[0].UTOCPath != filepath.Join(root, "A.utoc") ||
				bundles[1].PAKPath != filepath.Join(root, "B.pak_crateoff") ||
				bundles[1].UTOCPath != filepath.Join(scratch, "1", "bundle.utoc") {
				t.Fatalf("unexpected bundle paths: %+v", bundles)
			}
			payload, err := os.ReadFile(filepath.Join(scratch, "1", "bundle.ucas"))
			if err != nil || string(payload) != "B.UCAS" {
				t.Fatalf("staged UCAS = %q, error = %v", payload, err)
			}
			wantLayouts := []uassettool.PatchCheckLayout{{Build: 200, UsmapPath: "new.usmap"}, {Build: 100, UsmapPath: "old.usmap"}}
			if !reflect.DeepEqual(params["patch_layouts"], wantLayouts) {
				t.Fatalf("patch_layouts = %+v, want %+v", params["patch_layouts"], wantLayouts)
			}
			data, err := json.Marshal(map[string]any{
				"game_build": 200,
				"bundles": []uassettool.PatchCheckVerdict{
					{UTOCPath: bundles[0].UTOCPath, Status: "current"},
					{UTOCPath: bundles[1].UTOCPath, Status: "partial", Assets: []uassettool.PatchCheckAsset{
						{Path: "One.uasset", Outcome: "updated", FromBuild: 100},
						{Path: "Two.uasset", Outcome: "blocked", Reason: "property removed"},
					}},
				},
			})
			if err != nil {
				return err
			}
			return json.Unmarshal(data, result)
		}), nil, nil
	}

	// Act
	plan, err := CheckModsForPatch(context.Background(), PatchUpdateRequest{ModRoot: root, Layouts: layouts}, launch)

	// Assert
	if err != nil {
		t.Fatalf("CheckModsForPatch() error = %v", err)
	}
	if starts != 1 || calls != 1 || plan.GameBuild != 200 || len(plan.Mods) != 2 {
		t.Fatalf("worker starts = %d, calls = %d, plan = %+v", starts, calls, plan)
	}
	if plan.Mods[0].Status != "current" || !plan.Mods[0].Enabled {
		t.Errorf("enabled mod = %+v, want current", plan.Mods[0])
	}
	mod := plan.Mods[1]
	if mod.Status != "partial" || mod.Enabled || mod.Updated != 1 || !reflect.DeepEqual(mod.FromBuilds, []int{100}) ||
		!reflect.DeepEqual(mod.Blocked, []string{"Two.uasset: property removed"}) || len(mod.Assets) != 2 {
		t.Errorf("disabled mod = %+v, want one update and one blocked asset", mod)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Errorf("check staging still exists or could not be checked: %v", err)
	}
}
