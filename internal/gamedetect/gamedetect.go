// Package gamedetect locates a Marvel Rivals installation and its mod
// library through per-store providers. Detection is read-only: creating a
// missing library folder is a separate, user-confirmed step.
package gamedetect

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProviderSteam names the Steam provider, the default detection provider.
const ProviderSteam = "steam"

// ProviderEpic names the Epic Games launcher provider.
const ProviderEpic = "epic"

// LibraryDirName is the mod-library folder name inside a Marvel Rivals
// installation's Paks directory, an Unreal Engine modding convention.
const LibraryDirName = "~mods"

// modLibraryDirPerm is the permission used for the library folder Cratebug
// creates on confirmation: owner-writable, world-readable and traversable,
// matching what the game itself creates for content directories.
const modLibraryDirPerm os.FileMode = 0o755

// Describes which of the three detection outcomes a Detection reports.
type DetectionState string

const (
	// StateLibraryFound means the game install exists and already has a mod library.
	StateLibraryFound DetectionState = "libraryFound"
	// StateInstallFound means the game install exists but has no mod library yet.
	StateInstallFound DetectionState = "installFound"
	// StateNotFound means no game installation could be located.
	StateNotFound DetectionState = "notFound"
)

// Reports the outcome of one provider's search. LibraryPath is set only for
// StateLibraryFound. PaksPath is the verified game-install Paks directory
// and is set whenever the install itself was found, with or without the
// library, so the create-library flow can act on exactly what was verified.
type Detection struct {
	State       DetectionState `json:"state"`
	LibraryPath string         `json:"libraryPath,omitempty"`
	PaksPath    string         `json:"paksPath,omitempty"`
}

// Provider locates one store's Marvel Rivals installation. Detect never
// writes to the filesystem.
type Provider interface {
	Name() string
	Detect() (Detection, error)
}

// providerNames lists the providers Cratebug registers. ValidProvider keeps
// persisted settings from ever naming a provider this build cannot use; a
// new provider is added here and to NewDefaultRegistry together.
var providerNames = map[string]bool{
	ProviderSteam: true,
	ProviderEpic:  true,
}

// ValidProvider reports whether name names a provider this build can detect
// through.
func ValidProvider(name string) bool {
	return providerNames[name]
}

// Creates the registry production detects through.
func NewDefaultRegistry() *Registry {
	return NewRegistry(NewSteamProvider(), NewEpicProvider())
}

// Registry dispatches detection and confirmed library creation to the
// provider matching a store name.
type Registry struct {
	providers map[string]Provider
}

// Creates a Registry from providers. A later provider replaces an earlier
// one with the same name; production always registers distinct names.
func NewRegistry(providers ...Provider) *Registry {
	registry := &Registry{providers: make(map[string]Provider, len(providers))}
	for _, provider := range providers {
		registry.providers[provider.Name()] = provider
	}
	return registry
}

// Detect runs the provider named by provider and reports its three-state
// outcome.
func (r *Registry) Detect(provider string) (Detection, error) {
	p, ok := r.providers[provider]
	if !ok {
		return Detection{}, fmt.Errorf("unknown library provider: %q", provider)
	}
	return p.Detect()
}

// CreateLibrary re-detects provider's installation and creates its missing
// mod-library folder inside the verified install. This is the one write
// Cratebug performs outside a configured mod root, and callers gate it
// behind an explicit user confirmation. Re-detecting rather than accepting
// a path keeps creation aimed only at an install the provider just
// verified. An install whose library already exists returns the existing
// path unchanged.
func (r *Registry) CreateLibrary(provider string) (string, error) {
	detection, err := r.Detect(provider)
	if err != nil {
		return "", err
	}
	if detection.State == StateNotFound {
		return "", fmt.Errorf("cannot create the mod library: no %s installation was found", provider)
	}

	libraryPath := filepath.Join(detection.PaksPath, LibraryDirName)
	if err := os.Mkdir(libraryPath, modLibraryDirPerm); err != nil {
		if !os.IsExist(err) {
			return "", fmt.Errorf("create the %s mod library directory: %w", provider, err)
		}
		info, statErr := os.Stat(libraryPath)
		if statErr != nil {
			return "", fmt.Errorf("check the %s mod library path: %w", provider, statErr)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("the %s mod library path is not a directory", provider)
		}
	}
	return libraryPath, nil
}
