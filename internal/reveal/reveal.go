// Package reveal resolves library paths so Cratebug can open them in the
// system file manager without the UI supplying raw filesystem paths.
package reveal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// Names a validated absolute path and whether File Explorer should select it.
type Target struct {
	Path       string
	SelectItem bool
}

// Opens one root-relative folder, or the library root when folder is empty.
func OpenFolder(modRoot, folder string) error {
	target, err := ResolveFolder(modRoot, folder)
	if err != nil {
		return err
	}
	return openPath(target)
}

// Opens File Explorer with the scanned mod selected, or its folder if no file exists.
func OpenMod(modRoot, entryID string) error {
	target, err := ResolveMod(modRoot, entryID)
	if err != nil {
		return err
	}
	return openPath(target)
}

// Resolves a root-relative folder to an existing directory inside the library.
func ResolveFolder(modRoot, folder string) (Target, error) {
	path, err := resolveUnderRoot(modRoot, folder)
	if err != nil {
		return Target{}, err
	}

	label := "folder"
	if folder == "" {
		label = "mod library folder"
	}
	if err := requireDirectory(path, label); err != nil {
		return Target{}, err
	}
	return Target{Path: path}, nil
}

// Resolves a scanned entry to an existing file or folder inside the library.
func ResolveMod(modRoot, entryID string) (Target, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Target{}, fmt.Errorf("scan mod library before opening File Explorer: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Target{}, err
	}

	relative, selectItem := revealRelative(entry)
	path, err := resolveUnderRoot(library.Root, relative)
	if err != nil {
		return Target{}, err
	}

	if selectItem {
		if err := requireExisting(path, "mod file"); err != nil {
			return Target{}, err
		}
		return Target{Path: path, SelectItem: true}, nil
	}

	if err := requireDirectory(path, "folder"); err != nil {
		return Target{}, err
	}
	return Target{Path: path}, nil
}

func resolveUnderRoot(modRoot, relative string) (string, error) {
	if strings.TrimSpace(modRoot) == "" {
		return "", fmt.Errorf("mod library folder is not set")
	}

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return "", fmt.Errorf("resolve mod root: %w", err)
	}

	if relative == "" {
		return root, nil
	}
	if !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", fmt.Errorf("path is outside the mod root: %q", relative)
	}

	path := filepath.Join(root, filepath.FromSlash(relative))
	if !containedInRoot(root, path) {
		return "", fmt.Errorf("path escapes the mod root: %q", relative)
	}
	return path, nil
}

func revealRelative(entry discovery.Entry) (string, bool) {
	if entry.PrimaryPath != "" {
		return entry.PrimaryPath, true
	}
	if entry.Sidecars.UTOC != "" {
		return entry.Sidecars.UTOC, true
	}
	if entry.Sidecars.UCAS != "" {
		return entry.Sidecars.UCAS, true
	}
	return entry.RelativeFolder, false
}

func findEntry(entries []discovery.Entry, entryID string) (discovery.Entry, error) {
	for _, entry := range entries {
		if entry.ID == entryID {
			return entry, nil
		}
	}
	return discovery.Entry{}, fmt.Errorf("mod is not present in the current scan: %q", entryID)
}

func containedInRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return filepath.IsLocal(relative)
}

func requireDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing: %q", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %q", label, path)
	}
	return nil
}

func requireExisting(path, label string) error {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing: %q", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}
	return nil
}
