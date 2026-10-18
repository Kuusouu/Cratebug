package main

import (
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/conflict"
	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/reveal"
)

// Scans, classifies, and detects conflicts in the mod library, and locates,
// creates, and reveals the library folder.
type LibraryService struct {
	*core
}

func newLibraryService(c *core) *LibraryService {
	return &LibraryService{core: c}
}

// Releases library directory handles while the Tools menu is open.
func (s *LibraryService) SetLibraryWatcherSuspended(suspended bool) error {
	if s.watcher == nil {
		return nil
	}
	return s.watcher.SetSuspended(suspended)
}

// Returns the read-only catalog discovered beneath modRoot.
func (s *LibraryService) ScanLibrary(modRoot string) (discovery.Library, error) {
	if s.watcher != nil {
		currentRoot := s.watcher.Root()
		targetRoot := strings.TrimSpace(modRoot)
		if targetRoot != "" {
			if abs, err := filepath.Abs(targetRoot); err == nil {
				targetRoot = abs
			}
		}
		if filepath.Clean(currentRoot) != filepath.Clean(targetRoot) {
			_ = s.watcher.SetRoot(modRoot)
		}
	}
	return discovery.Scan(modRoot)
}

// ClassifyLibrary determines the category and hero/skin identity for discovered entries.
func (s *LibraryService) ClassifyLibrary(modRoot string, entries []discovery.Entry) (map[string]modtype.Identity, error) {
	return s.catalog.classify(modRoot, entries)
}

// DetectConflicts scans entries for enabled mods that share internal Unreal
// asset paths. It classifies entries first (a no-op for anything already
// classified this session, since internal/modtype's SessionClassifier keyed
// on Classify's own mtime-based cache also skips the corresponding
// UAssetToolRivals call), then reuses each enabled mod's retained path
// listing from that same cache rather than resolving it a second time.
func (s *LibraryService) DetectConflicts(modRoot string, entries []discovery.Entry) (conflict.Result, error) {
	if _, err := s.catalog.classify(modRoot, entries); err != nil {
		return conflict.Result{}, err
	}

	paths := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if entry.State != discovery.StateEnabled {
			continue
		}
		if resolved, ok := s.catalog.pathsFor(modRoot, entry); ok {
			paths[entry.ID] = resolved
		}
	}

	return conflict.Detect(entries, paths), nil
}

// Persists the selected mod root so the library does not need to be
// reselected the next time Cratebug launches.
func (s *LibraryService) SetModRoot(modRoot string) error {
	if s.watcher != nil {
		_ = s.watcher.SetRoot(modRoot)
	}
	return s.meta.update(func(doc *metadata.Document) error {
		doc.Settings.ModRoot = modRoot
		return nil
	})
}

// DetectLibrary locates the named store provider's Marvel Rivals
// installation and mod library. Read-only: nothing is written for any
// detection outcome.
func (s *LibraryService) DetectLibrary(provider string) (gamedetect.Detection, error) {
	return s.detector.Detect(provider)
}

// CreateLibrary re-detects the named store provider's installation and
// creates its missing mod-library folder, the one write Cratebug performs
// outside a configured mod root. The frontend calls it only after the user
// confirmed the creation in a dialog.
func (s *LibraryService) CreateLibrary(provider string) (string, error) {
	return s.detector.CreateLibrary(provider)
}

// Opens one root-relative folder, or the library root, in File Explorer.
func (s *LibraryService) OpenFolderInExplorer(modRoot, folder string) error {
	return reveal.OpenFolder(modRoot, folder)
}

// Opens File Explorer with the scanned mod selected.
func (s *LibraryService) OpenModInExplorer(modRoot, entryID string) error {
	return reveal.OpenMod(modRoot, entryID)
}
