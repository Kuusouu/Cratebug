package patchupdate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// BundleStatus summarizes what an update did to one IoStore bundle.
type BundleStatus string

const (
	// Nothing in the bundle needed an update.
	BundleCurrent BundleStatus = "current"
	// Every asset that needed an update got one.
	BundleUpdated BundleStatus = "updated"
	// Some assets were updated and others could not be.
	BundlePartial BundleStatus = "partial"
	// Assets need an update the updater cannot make, and nothing changed.
	BundleBlocked BundleStatus = "blocked"
	// The bundle's layout could not be told, so it was left alone.
	BundleAmbiguous BundleStatus = "ambiguous"
)

// Extracted files that must come back byte for byte after the rebuild.
// Headers (.uasset) are regenerated from the IoStore package and may differ.
var verbatimExtensions = map[string]bool{".uexp": true, ".ubulk": true, ".uptnl": true}

// BundleOptions controls one bundle update.
type BundleOptions struct {
	// The game's Paks directory, used to resolve the mod's references.
	GamePaks string
	// Re-encrypt the rebuilt container with the game key.
	Encrypt bool
	// Names the rebuilt container. The container ID derives from it, so it
	// must differ between mods.
	ContainerName string
}

// BundleResult reports one bundle update. The output paths are set only when
// assets changed, and point at verified files inside the work directory.
type BundleResult struct {
	Status BundleStatus  `json:"status"`
	Assets []AssetResult `json:"assets"`
	Detail string        `json:"detail,omitempty"`
	PAK    string        `json:"-"`
	UTOC   string        `json:"-"`
	UCAS   string        `json:"-"`
}

// UpdateBundle brings the bundle at pakPath and utocPath into the current
// layout inside workDir. The source files are only read.
func UpdateBundle(c Caller, layouts *Layouts, pakPath, utocPath, workDir string, options BundleOptions) (BundleResult, error) {
	if options.GamePaks == "" || options.ContainerName == "" {
		return BundleResult{}, fmt.Errorf("patchupdate: game paks and container name are required")
	}

	companion, err := uassettool.ListPakWithKey(c, pakPath, uassettool.MarvelRivalsAESKey)
	if err != nil {
		return BundleResult{}, fmt.Errorf("list companion PAK: %w", err)
	}
	if uassettool.CompanionPakHasRawFiles(companion) {
		return BundleResult{
			Status: BundleBlocked,
			Detail: "keeps loose files in its .pak, which the updater does not rebuild yet",
		}, nil
	}

	legacy := filepath.Join(workDir, "legacy")
	if err := extractLegacy(c, options.GamePaks, utocPath, legacy); err != nil {
		return BundleResult{}, err
	}
	original, err := snapshot(legacy)
	if err != nil {
		return BundleResult{}, err
	}

	result, sources, err := updateAssets(c, layouts, legacy, filepath.Join(workDir, "assets"))
	if err != nil {
		return BundleResult{}, err
	}
	if len(sources) == 0 {
		return result, nil
	}

	built := filepath.Join(workDir, "built", options.ContainerName)
	if err := os.MkdirAll(filepath.Dir(built), 0o700); err != nil {
		return BundleResult{}, err
	}
	if _, err := uassettool.CreateModIoStore(c, built, legacy, uassettool.IoStoreCreateOptions{Compress: true}); err != nil {
		return BundleResult{}, fmt.Errorf("rebuild container: %w", err)
	}
	result.PAK, result.UTOC, result.UCAS = built+".pak", built+".utoc", built+".ucas"

	if err := sameListing(c, utocPath, result.UTOC); err != nil {
		return BundleResult{}, err
	}
	if err := cleanCompanion(c, result.PAK); err != nil {
		return BundleResult{}, err
	}
	if options.Encrypt {
		if _, err := uassettool.EncryptIoStoreDirect(c, result.PAK, uassettool.MarvelRivalsAESKey); err != nil {
			return BundleResult{}, fmt.Errorf("encrypt rebuilt container: %w", err)
		}
	}
	encrypted, err := uassettool.IsIoStoreEncrypted(c, result.UTOC)
	if err != nil {
		return BundleResult{}, fmt.Errorf("check rebuilt encryption: %w", err)
	}
	if encrypted != options.Encrypt {
		return BundleResult{}, fmt.Errorf("rebuilt container did not reach the requested encryption state")
	}

	if err := verifyRebuild(c, layouts, options.GamePaks, result.UTOC, filepath.Join(workDir, "verify"), original, legacy, sources); err != nil {
		return BundleResult{}, fmt.Errorf("verify rebuilt container: %w", err)
	}
	return result, nil
}

// Runs every legacy asset through the updater and returns, for each updated
// one, its reading in the layout it was cooked with.
func updateAssets(c Caller, layouts *Layouts, legacy, scratch string) (BundleResult, map[string]assetRead, error) {
	var uassets []string
	err := filepath.WalkDir(legacy, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".uasset") {
			uassets = append(uassets, path)
		}
		return err
	})
	if err != nil {
		return BundleResult{}, nil, fmt.Errorf("list extracted assets: %w", err)
	}
	sort.Strings(uassets)

	result := BundleResult{Status: BundleCurrent}
	sources := map[string]assetRead{}
	updated, blocked, ambiguous := 0, 0, 0
	for index, uasset := range uassets {
		job := assetJob{
			caller:  c,
			layouts: layouts,
			uasset:  uasset,
			scratch: filepath.Join(scratch, strconv.Itoa(index)),
		}
		if err := os.MkdirAll(job.scratch, 0o700); err != nil {
			return BundleResult{}, nil, err
		}
		asset, source, err := job.run()
		if err != nil {
			return BundleResult{}, nil, fmt.Errorf("%s: %w", relative(legacy, uasset), err)
		}
		asset.Path = relative(legacy, uasset)
		result.Assets = append(result.Assets, asset)
		switch asset.Outcome {
		case AssetUpdated:
			updated++
			sources[uasset] = source
		case AssetBlocked:
			blocked++
		case AssetAmbiguous:
			ambiguous++
		}
	}

	result.Status = bundleStatus(updated, blocked, ambiguous)
	return result, sources, nil
}

// Summarizes per-asset outcomes. An ambiguous asset is not a failure to
// apply; it is an asset whose layout could not be told, so it stays as is.
func bundleStatus(updated, blocked, ambiguous int) BundleStatus {
	switch {
	case updated > 0 && blocked > 0:
		return BundlePartial
	case updated > 0:
		return BundleUpdated
	case blocked > 0:
		return BundleBlocked
	case ambiguous > 0:
		return BundleAmbiguous
	default:
		return BundleCurrent
	}
}

func extractLegacy(c Caller, gamePaks, utocPath, output string) error {
	if err := os.MkdirAll(output, 0o700); err != nil {
		return err
	}
	var extracted struct {
		ExtractedCount int `json:"extracted_count"`
		FailedCount    int `json:"failed_count"`
	}
	err := c.Call("extract_iostore_legacy", map[string]any{
		"game_paks":   gamePaks,
		"mod_path":    utocPath,
		"output_path": output,
		"aes_key":     uassettool.MarvelRivalsAESKey,
	}, &extracted)
	if err != nil {
		return fmt.Errorf("extract packages: %w", err)
	}
	if extracted.FailedCount != 0 {
		return fmt.Errorf("extract packages: %d of %d failed", extracted.FailedCount, extracted.FailedCount+extracted.ExtractedCount)
	}
	return nil
}

// Contents of every file under root, by path relative to root.
func snapshot(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relative(root, path)] = data
		return nil
	})
	return files, err
}

func sameListing(c Caller, before, after string) error {
	want, err := uassettool.ListIoStoreFiles(c, before, uassettool.MarvelRivalsAESKey)
	if err != nil {
		return fmt.Errorf("list original container: %w", err)
	}
	got, err := uassettool.ListIoStoreFiles(c, after, uassettool.MarvelRivalsAESKey)
	if err != nil {
		return fmt.Errorf("list rebuilt container: %w", err)
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		return fmt.Errorf("rebuilt container lists %d files, the original %d", len(got), len(want))
	}
	return nil
}

func cleanCompanion(c Caller, pakPath string) error {
	listing, err := uassettool.ListPakWithKey(c, pakPath, uassettool.MarvelRivalsAESKey)
	if err != nil {
		return fmt.Errorf("list rebuilt companion PAK: %w", err)
	}
	if unsupported := uassettool.CompanionPakUnsupportedPaths(listing); len(unsupported) > 0 {
		return fmt.Errorf("rebuilt companion PAK contains %s", strings.Join(unsupported, ", "))
	}
	return nil
}

// Extracts the rebuilt container again and checks it end to end: every
// payload file is byte-identical to what went in, and each updated asset
// reads cleanly in the current layout with its original values.
func verifyRebuild(c Caller, layouts *Layouts, gamePaks, utocPath, output string, original map[string][]byte, legacy string, sources map[string]assetRead) error {
	if err := extractLegacy(c, gamePaks, utocPath, output); err != nil {
		return err
	}
	rebuilt, err := snapshot(output)
	if err != nil {
		return err
	}
	expected, err := snapshot(legacy)
	if err != nil {
		return err
	}
	if len(rebuilt) != len(original) {
		return fmt.Errorf("extracted %d files, expected %d", len(rebuilt), len(original))
	}
	for path, want := range expected {
		got, ok := rebuilt[path]
		if !ok {
			return fmt.Errorf("%s is missing", path)
		}
		if verbatimExtensions[strings.ToLower(filepath.Ext(path))] && !bytes.Equal(got, want) {
			return fmt.Errorf("%s changed in the rebuild", path)
		}
	}

	scratch := filepath.Join(output, "..", "verify-reads")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		return err
	}
	for uasset, source := range sources {
		job := assetJob{caller: c, layouts: layouts, scratch: scratch}
		check, err := job.readFile(filepath.Join(output, relative(legacy, uasset)), layouts.Current, "rebuilt")
		if err != nil {
			return err
		}
		if !check.clean || !check.sameValues(source) {
			return fmt.Errorf("%s does not read back with its original values", relative(legacy, uasset))
		}
	}
	return nil
}

func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
