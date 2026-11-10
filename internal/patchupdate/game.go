package patchupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/Kuusouu/Cratebug/internal/usmap"
)

const (
	// The global container every Marvel Rivals Paks directory holds.
	globalContainerName = "global.utoc"
	// How far above the mod root the Paks directory may sit (Paks/~mods/...).
	maxPaksSearchDepth = 3
	// Earlier layouts loaded to recognize what a mod was cooked against.
	// Each is about 1.3 MB; a dozen reaches back a few months of patches.
	DefaultPreviousLayouts = 12
)

// Patch containers carry the game build: Patch_-Windows_1.1.3984769_P.utoc.
var patchContainerPattern = regexp.MustCompile(`(?i)^Patch_-Windows_[0-9]+\.[0-9]+\.([0-9]+)_P\.(utoc|pak)$`)

// ErrGamePaksNotFound means the mod root is not inside a game install.
var ErrGamePaksNotFound = errors.New("the game's Paks folder was not found above the mod folder")

// ErrGameBuildUnknown means the Paks folder has no patch container to date it.
var ErrGameBuildUnknown = errors.New("the game build could not be read from its Paks folder")

// FindGamePaks returns the Paks directory that contains modRoot.
func FindGamePaks(modRoot string) (string, error) {
	current, err := filepath.Abs(modRoot)
	if err != nil {
		return "", err
	}
	for depth := 0; depth <= maxPaksSearchDepth; depth++ {
		if info, err := os.Stat(filepath.Join(current, globalContainerName)); err == nil && info.Mode().IsRegular() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", ErrGamePaksNotFound
}

// GameBuild returns the newest build among the patch containers in paksDir.
func GameBuild(paksDir string) (int, error) {
	entries, err := os.ReadDir(paksDir)
	if err != nil {
		return 0, fmt.Errorf("read Paks folder: %w", err)
	}
	build := 0
	for _, entry := range entries {
		match := patchContainerPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if value, err := strconv.Atoi(match[1]); err == nil && value > build {
			build = value
		}
	}
	if build == 0 {
		return 0, ErrGameBuildUnknown
	}
	return build, nil
}

// LoadLayouts fetches the mappings for gameBuild and up to previous earlier
// builds. When the depot cannot be reached it falls back to cached files.
func LoadLayouts(ctx context.Context, depot Depot, gameBuild, previous int) (*Layouts, error) {
	files, err := depot.List(ctx)
	if err != nil {
		cached, cacheErr := depot.Cached()
		if cacheErr != nil || len(cached) == 0 {
			return nil, err
		}
		files = cached
	}

	var current *MappingsFile
	var older []MappingsFile
	for i := range files {
		switch {
		case files[i].Build == gameBuild:
			current = &files[i]
		case files[i].Build < gameBuild && len(older) < previous:
			older = append(older, files[i])
		}
	}
	if current == nil {
		return nil, fmt.Errorf("%w (build %d)", ErrMappingsNotPublished, gameBuild)
	}

	load := func(file MappingsFile) (Layout, error) {
		path, err := depot.Fetch(ctx, file)
		if err != nil {
			return Layout{}, err
		}
		mappings, err := usmap.Load(path)
		if err != nil {
			return Layout{}, err
		}
		return Layout{Build: file.Build, Path: path, Mappings: mappings}, nil
	}
	currentLayout, err := load(*current)
	if err != nil {
		return nil, err
	}
	var olderLayouts []Layout
	for _, file := range older {
		layout, err := load(file)
		if err != nil {
			// An older layout only widens what can be recognized; skip it.
			continue
		}
		olderLayouts = append(olderLayouts, layout)
	}
	return NewLayouts(currentLayout, olderLayouts), nil
}
