package gamedetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Steam's install location is recorded in this registry value. Steam writes
// it with forward slashes, so it passes through filepath.FromSlash before
// use.
const steamRegistryKey = `Software\Valve\Steam`
const steamRegistryValue = "SteamPath"

// steamVDFRelativePath is the Steam library list Steam maintains beneath
// every install root.
const steamVDFRelativePath = `steamapps\libraryfolders.vdf`

// marvelPaksRelativePath is the game-install shape beneath a Steam library,
// verified to exist before an install is reported as found.
var marvelPaksRelativePath = filepath.Join("steamapps", "common", "MarvelRivals", "MarvelGame", "Marvel", "Content", "Paks")

// defaultSteamFallbackRoots are Steam install locations checked when the
// registry has no answer, covering the common cases Steam itself uses.
// BentoMod's detection (the behavioral reference) ships the same list.
var defaultSteamFallbackRoots = []string{
	`C:\Program Files (x86)\Steam`,
	`C:\Program Files\Steam`,
	`D:\Steam`,
	`D:\Program Files (x86)\Steam`,
	`D:\Program Files\Steam`,
	`E:\Steam`,
	`E:\SteamLibrary`,
	`F:\Steam`,
	`F:\SteamLibrary`,
}

// NewSteamProvider creates the production Steam provider reading the real
// registry.
func NewSteamProvider() SteamProvider {
	return SteamProvider{
		registrySteamPath: defaultSteamPath,
		fallbackRoots:     defaultSteamFallbackRoots,
	}
}

// SteamProvider locates Marvel Rivals through Steam's own install records:
// the registry-named install root and each library listed in a
// libraryfolders.vdf.
type SteamProvider struct {
	// registrySteamPath resolves Steam's install root. Tests replace it so
	// no real registry or install is touched.
	registrySteamPath func() (string, error)
	// fallbackRoots are additional Steam install roots whose library lists
	// are consulted when the registry has no answer.
	fallbackRoots []string
}

// Names the provider as recorded in settings.
func (p SteamProvider) Name() string {
	return ProviderSteam
}

// Detect walks every Steam library it can resolve, in registry-then-fallback
// order, and reports the first one holding a Marvel Rivals installation.
func (p SteamProvider) Detect() (Detection, error) {
	for _, libraryRoot := range p.libraryRoots() {
		paksPath := filepath.Join(libraryRoot, marvelPaksRelativePath)
		if !isDir(paksPath) {
			continue
		}

		libraryPath := filepath.Join(paksPath, LibraryDirName)
		if isDir(libraryPath) {
			return Detection{
				State:       StateLibraryFound,
				LibraryPath: libraryPath,
				PaksPath:    paksPath,
			}, nil
		}
		return Detection{State: StateInstallFound, PaksPath: paksPath}, nil
	}
	return Detection{State: StateNotFound}, nil
}

// libraryRoots resolves every Steam library root to check: the
// registry-named install root and the fallback locations first, then every
// library their libraryfolders.vdf files list. A missing registry entry or
// unreadable file list is not an error; the remaining candidates still get
// their chance. Near-duplicates (differing only in path case or slash
// direction) are dropped with a case-insensitive comparison, since these
// are Windows paths.
func (p SteamProvider) libraryRoots() []string {
	var roots []string
	add := func(path string) {
		normalized := filepath.Clean(path)
		if normalized == "." {
			return
		}
		for _, existing := range roots {
			if strings.EqualFold(existing, normalized) {
				return
			}
		}
		roots = append(roots, normalized)
	}

	if p.registrySteamPath != nil {
		if steamRoot, err := p.registrySteamPath(); err == nil && steamRoot != "" {
			add(steamRoot)
		}
	}
	for _, fallback := range p.fallbackRoots {
		add(fallback)
	}

	for _, root := range append([]string(nil), roots...) {
		content, err := os.ReadFile(filepath.Join(root, steamVDFRelativePath))
		if err != nil {
			continue
		}
		for _, library := range parseSteamLibraryPaths(string(content)) {
			add(library)
		}
	}
	return roots
}

// defaultSteamPath reads Steam's install root from the registry.
func defaultSteamPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, steamRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open the Steam registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(steamRegistryValue)
	if err != nil {
		return "", fmt.Errorf("read the %s registry value: %w", steamRegistryValue, err)
	}
	return filepath.FromSlash(value), nil
}

// parseSteamLibraryPaths extracts each "path" entry from a
// libraryfolders.vdf in file order. Steam escapes path separators as \\, so
// values are unescaped before use.
func parseSteamLibraryPaths(vdfContent string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(vdfContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, `"path"`) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, `"path"`))
		value = strings.Trim(value, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		paths = append(paths, value)
	}
	return paths
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
