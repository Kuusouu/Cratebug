package gamedetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Marvel Rivals' Epic Games catalog product id, verified against a live
// launcher .item on 2026-09-02. The install folder name itself carries a
// random suffix and is not a match key.
const epicMarvelCatalogNamespace = "38e211ced4e448a5a653a8d1e13fef18"

// Display-name fallback when a SKU uses a different catalog namespace.
const epicMarvelDisplayName = "Marvel Rivals"

// Game-install shape beneath an Epic InstallLocation. Steam's
// steamapps/common/MarvelRivals prefix is not present here.
var epicPaksRelativePath = filepath.Join("MarvelGame", "Marvel", "Content", "Paks")

// Locates Marvel Rivals through the Epic Games launcher's per-app .item
// manifests. Detect never writes.
type EpicProvider struct {
	// Launcher Manifests folder. Tests replace it so no real ProgramData
	// or game install is touched.
	manifestsDir string
}

type epicItem struct {
	IncompleteInstall bool   `json:"bIsIncompleteInstall"`
	DisplayName       string `json:"DisplayName"`
	InstallLocation   string `json:"InstallLocation"`
	CatalogNamespace  string `json:"CatalogNamespace"`
	MainGameAppName   string `json:"MainGameAppName"`
}

// Production provider that reads the live Epic Games launcher manifests.
func NewEpicProvider() EpicProvider {
	return EpicProvider{manifestsDir: defaultEpicManifestsDir()}
}

// Names the provider as recorded in settings.
func (p EpicProvider) Name() string {
	return ProviderEpic
}

// Reports the first Marvel Rivals install whose Paks directory exists,
// walking launcher .item files in sorted filename order.
func (p EpicProvider) Detect() (Detection, error) {
	for _, name := range p.itemFilenames() {
		item, ok := readEpicItem(filepath.Join(p.manifestsDir, name))
		if !ok || !isMarvelRivalsItem(item) {
			continue
		}

		detection, ok := detectionForInstall(item.InstallLocation)
		if !ok {
			continue
		}
		return detection, nil
	}
	return Detection{State: StateNotFound}, nil
}

func defaultEpicManifestsDir() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return ""
	}
	return filepath.Join(programData, "Epic", "EpicGamesLauncher", "Data", "Manifests")
}

func (p EpicProvider) itemFilenames() []string {
	if p.manifestsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(p.manifestsDir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".item") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func readEpicItem(path string) (epicItem, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return epicItem{}, false
	}

	var item epicItem
	if err := json.Unmarshal(content, &item); err != nil {
		return epicItem{}, false
	}
	return item, true
}

func isMarvelRivalsItem(item epicItem) bool {
	if item.IncompleteInstall {
		return false
	}
	// DLC items name the parent game here; their InstallLocation is not
	// the full game.
	if item.MainGameAppName != "" {
		return false
	}
	if item.CatalogNamespace == epicMarvelCatalogNamespace {
		return true
	}
	return strings.EqualFold(item.DisplayName, epicMarvelDisplayName)
}

func detectionForInstall(installLocation string) (Detection, bool) {
	if installLocation == "" {
		return Detection{}, false
	}

	paksPath := filepath.Join(installLocation, epicPaksRelativePath)
	if !isDir(paksPath) {
		return Detection{}, false
	}

	libraryPath := filepath.Join(paksPath, LibraryDirName)
	if isDir(libraryPath) {
		return Detection{
			State:       StateLibraryFound,
			LibraryPath: libraryPath,
			PaksPath:    paksPath,
		}, true
	}
	return Detection{State: StateInstallFound, PaksPath: paksPath}, true
}
