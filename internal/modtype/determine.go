package modtype

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Reported when a mod's type cannot be resolved without a capability
// Cratebug does not have yet. Currently this is only an encrypted IoStore
// container: Cratebug does not manage AES keys for encrypted mods, so its
// contents cannot be listed.
var ErrCannotDetermineType = errors.New("modtype: cannot determine type")

// Sends one worker request and decodes its data payload into result.
// uassettool.Adapter and uassettool.Worker both satisfy this structurally.
type caller interface {
	Call(action string, params map[string]any, result any) error
}

// Resolves entry's coarse category using only cheap, header-only
// UAssetToolRivals facts (is_iostore_encrypted) plus a single internal-path
// listing call (list_pak or list_iostore_files) run through Classify —
// never extract_iostore or detect_type. root is the discovery.Library.Root
// entry's paths are relative to.
//
// Determine returns ErrCannotDetermineType, not a lower-level error, when
// the mod is an encrypted IoStore container or otherwise has no listable
// content; callers should treat that as "unknown for now", not a failure.
//
// This package ships no pooling of its own: run Determine sequentially
// against one reused worker for routine per-mod use. A caller classifying
// a large library (hundreds of entries or more) may still want to pool; see
// uassettool.WorkerPoolSizeForLibrary and the entry-count-tiered policy in
// docs/decisions/0003-uassettoolrivals-boundary.md. Each pooled worker needs
// its own Worker instance, since Worker.Call is not safe for concurrent use.
func Determine(c caller, root string, entry discovery.Entry) (Category, error) {
	paths, err := listInternalPaths(c, root, entry)
	if err != nil {
		return "", err
	}
	return Classify(paths), nil
}

// Resolves entry's list of internal asset paths: the single listing call
// Determine and DetermineIdentity both build on, so hero/skin resolution
// (task 6.8) needs no UAssetToolRivals call beyond what category
// classification already performs.
func listInternalPaths(c caller, root string, entry discovery.Entry) ([]string, error) {
	if entry.Kind != discovery.EntryMod {
		return nil, fmt.Errorf("modtype: entry %q is not a mod", entry.ID)
	}

	switch entry.BundleFormat {
	case discovery.BundleFormatClassic:
		listing, err := uassettool.ListPak(c, absPath(root, entry.PrimaryPath))
		if err != nil {
			return nil, err
		}

		paths := make([]string, len(listing))
		for i, file := range listing {
			paths[i] = file.Path
		}
		return paths, nil

	case discovery.BundleFormatIoStore:
		if entry.Sidecars.UTOC == "" {
			return nil, fmt.Errorf("%w: %q has no .utoc sidecar to list", ErrCannotDetermineType, entry.ID)
		}

		utocPath := absPath(root, entry.Sidecars.UTOC)
		encrypted, err := uassettool.IsIoStoreEncrypted(c, utocPath)
		if err != nil {
			return nil, err
		}
		if encrypted {
			return nil, fmt.Errorf("%w: %q is an encrypted IoStore container", ErrCannotDetermineType, entry.ID)
		}

		return uassettool.ListIoStoreFiles(c, utocPath, "")

	default:
		return nil, fmt.Errorf("%w: %q has no recognized bundle format", ErrCannotDetermineType, entry.ID)
	}
}

// Resolves a discovery-relative, forward-slash-normalized path against root.
func absPath(root, relative string) string {
	return filepath.Join(root, filepath.FromSlash(relative))
}
