package patchupdate

import "testing"

type summaryCaller struct {
	calls int
	has   bool
}

func (c *summaryCaller) Call(action string, _ map[string]any, result any) error {
	c.calls++
	if action != "get_asset_summary" {
		return nil
	}
	if summary, ok := result.(*assetSummary); ok {
		summary.HasUnversionedProperties = c.has
	}
	return nil
}

func TestAssetJobSkipsVersionedAssetsWithoutConvertingThem(t *testing.T) {
	// Arrange
	caller := &summaryCaller{has: false}
	job := assetJob{
		caller:  caller,
		layouts: &Layouts{Current: Layout{Path: "current.usmap"}},
		uasset:  "asset.uasset",
	}

	// Act
	result, _, err := job.run()

	// Assert
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if result.Outcome != AssetCurrent {
		t.Errorf("outcome = %q, want %q", result.Outcome, AssetCurrent)
	}
	if caller.calls != 1 {
		t.Errorf("worker calls = %d, want only the summary check", caller.calls)
	}
}
