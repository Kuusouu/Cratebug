package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"golang.org/x/sys/windows"
)

// Moves one current scanner entry to an existing physical folder.
func moveMod(modRoot, entryID, destinationFolder string) (Result, error) {
	return moveModWithMove(modRoot, entryID, destinationFolder, moveFileWithoutReplace)
}

// Allows bundle-move tests to inject a later filesystem failure and exercise
// the same rollback and reconciliation path as production code.
func moveModWithMove(modRoot, entryID, destinationFolder string, move func(string, string) error) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before move: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Result{}, err
	}

	destinationFolder, err = knownFolder(library.Folders, destinationFolder)
	if err != nil {
		return Result{}, err
	}

	stem, err := movedStem(entry, destinationFolder)
	if err != nil {
		return Result{}, err
	}
	plan, err := buildBundlePlan(library.Root, entry, stem)
	if err != nil {
		return Result{}, err
	}

	if err := requireBundleDirectoryAncestry(library.Root, plan); err != nil {
		return Result{}, err
	}

	if err := plan.apply(move); err != nil {
		return reconcileFailedPlan(modRoot, entry.ID, entry.PrimaryPath, plan, err)
	}
	return reconcileResult(modRoot, entry.ID, entry.PrimaryPath, plan.destinationPrimary)
}

// Creates one physical folder below a scanner-known parent folder or the root.
func createFolder(modRoot, parentFolder, name string) (Result, error) {
	return createFolderWithDirectoryCheck(modRoot, parentFolder, name, requireDirectory)
}

// Allows directory-safety tests to simulate a parent that changes after scan.
func createFolderWithDirectoryCheck(modRoot, parentFolder, name string, requireDirectoryFunc func(string, string) error) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before creating folder: %w", err)
	}

	return createFolderFromLibrary(library, parentFolder, name, requireDirectoryFunc)
}

// Applies folder-creation validation to one scan result. Keeping the scan and
// mutation stages separate makes the post-scan ancestry check explicit.
func createFolderFromLibrary(library discovery.Library, parentFolder, name string, requireDirectoryFunc func(string, string) error) (Result, error) {
	parentFolder, err := knownFolder(library.Folders, parentFolder)
	if err != nil {
		return Result{}, err
	}
	if err := validateFileStem(name); err != nil {
		return Result{}, fmt.Errorf("folder name: %w", err)
	}

	// Joining a validated leaf name to the scanner's canonical parent prevents
	// the frontend from choosing a path outside the scanned library.
	folderPath := filepath.ToSlash(filepath.Join(parentFolder, name))
	root, err := filepath.Abs(library.Root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve mod root: %w", err)
	}

	absolutePath, err := resolveFolderPath(root, folderPath)
	if err != nil {
		return Result{}, err
	}

	if err := requireDirectoryAncestry(root, filepath.Dir(absolutePath), requireDirectoryFunc); err != nil {
		return Result{}, err
	}
	if _, err := os.Lstat(absolutePath); err == nil {
		return Result{}, fmt.Errorf("folder already exists: %q", folderPath)
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("check destination folder: %w", err)
	}

	if err := os.Mkdir(absolutePath, 0o700); err != nil {
		return Result{}, fmt.Errorf("create folder %q: %w", folderPath, err)
	}

	if err := requireDirectoryFunc(absolutePath, "created folder"); err != nil {
		return Result{}, fmt.Errorf("folder creation completed but reconciliation failed: %w", err)
	}
	return Result{FolderPath: folderPath}, nil
}

// Renames one scanner-known physical folder without allowing root mutation.
func renameFolder(modRoot, folder, name string) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before renaming folder: %w", err)
	}
	folder, err = mutableKnownFolder(library.Folders, folder)
	if err != nil {
		return Result{}, err
	}
	if err := validateFileStem(name); err != nil {
		return Result{}, fmt.Errorf("folder name: %w", err)
	}

	destinationFolder := filepath.ToSlash(filepath.Join(folderParent(folder), name))
	return moveFolderWithMove(library.Root, library.Folders, folder, destinationFolder, moveFileWithoutReplace)
}

// Moves one scanner-known folder below an existing physical folder or the root.
func moveFolderToParent(modRoot, folder, destinationParent string) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before moving folder: %w", err)
	}
	folder, err = mutableKnownFolder(library.Folders, folder)
	if err != nil {
		return Result{}, err
	}
	destinationParent, err = knownFolder(library.Folders, destinationParent)
	if err != nil {
		return Result{}, err
	}

	destinationFolder := filepath.ToSlash(filepath.Join(destinationParent, filepath.Base(folder)))
	return moveFolderWithMove(library.Root, library.Folders, folder, destinationFolder, moveFileWithoutReplace)
}

// Applies a no-replace directory move and returns the folder paths needed for
// targeted UI reconciliation.
func moveFolder(modRoot string, folders []string, sourceFolder, destinationFolder string) (Result, error) {
	return moveFolderWithMove(modRoot, folders, sourceFolder, destinationFolder, moveFileWithoutReplace)
}

// Allows folder-move tests to inject a filesystem failure after planning and
// exercise the same reconciliation path as production code.
func moveFolderWithMove(modRoot string, folders []string, sourceFolder, destinationFolder string, move func(string, string) error) (Result, error) {
	return moveFolderWithFunctions(modRoot, folders, sourceFolder, destinationFolder, move, requireDirectory)
}

// Keeps post-move reconciliation testable without changing the production
// directory validation policy.
func moveFolderWithFunctions(
	modRoot string,
	folders []string,
	sourceFolder string,
	destinationFolder string,
	move func(string, string) error,
	requireDirectoryFunc func(string, string) error,
) (Result, error) {
	sourceFolder, err := mutableKnownFolder(folders, sourceFolder)
	if err != nil {
		return Result{}, err
	}

	destinationParent, err := knownFolder(folders, folderParent(destinationFolder))
	if err != nil {
		return Result{}, err
	}

	destinationFolder = filepath.ToSlash(filepath.Join(destinationParent, filepath.Base(destinationFolder)))
	if strings.EqualFold(sourceFolder, destinationFolder) {
		return Result{}, fmt.Errorf("folder destination is unchanged: %q", sourceFolder)
	}

	if isDescendantFolder(destinationFolder, sourceFolder) {
		return Result{}, fmt.Errorf("cannot move folder %q into itself or a descendant", sourceFolder)
	}

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("resolve mod root: %w", err)
	}

	source, err := resolveFolderPath(root, sourceFolder)
	if err != nil {
		return Result{}, err
	}

	destination, err := resolveFolderPath(root, destinationFolder)
	if err != nil {
		return Result{}, err
	}

	if err := requireDirectoryAncestry(root, filepath.Dir(source), requireDirectoryFunc); err != nil {
		return Result{}, err
	}

	if err := requireDirectoryAncestry(root, filepath.Dir(destination), requireDirectoryFunc); err != nil {
		return Result{}, err
	}
	if err := requireDirectoryFunc(source, "source folder"); err != nil {
		return Result{}, err
	}

	if _, err := os.Lstat(destination); err == nil {
		return Result{}, fmt.Errorf("destination folder already exists: %q", destinationFolder)
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("check destination folder: %w", err)
	}

	if err := move(source, destination); err != nil {
		return reconcileFailedFolder(root, sourceFolder, destinationFolder, err)
	}

	if destinationErr := requireDirectoryFunc(destination, "moved folder"); destinationErr != nil {
		return reconcileFailedFolder(root, sourceFolder, destinationFolder, destinationErr)
	}

	if _, err := os.Lstat(source); err == nil {
		return reconcileFailedFolder(root, sourceFolder, destinationFolder, fmt.Errorf("folder move completed but source still exists: %q", sourceFolder))
	} else if !os.IsNotExist(err) {
		return reconcileFailedFolder(root, sourceFolder, destinationFolder, fmt.Errorf("reconcile source folder: %w", err))
	}

	return Result{PreviousFolderPath: sourceFolder, FolderPath: destinationFolder}, nil
}

// Validates all existing directories used by a bundle plan immediately before
// its moves begin. File paths can be scanner-known while a parent junction has
// changed since scanning, so both source and destination ancestry are checked.
func requireBundleDirectoryAncestry(root string, plan bundlePlan) error {
	for _, move := range plan.moves {
		if err := requireDirectoryAncestry(root, filepath.Dir(move.source), requireDirectory); err != nil {
			return err
		}

		if err := requireDirectoryAncestry(root, filepath.Dir(move.destination), requireDirectory); err != nil {
			return err
		}
	}
	return nil
}

// Resolves a root-relative folder path and rejects root itself as a mutation target.
func resolveFolderPath(root, relative string) (string, error) {
	if relative == "" || !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", fmt.Errorf("folder path is outside the mod root: %q", relative)
	}

	path := filepath.Join(root, filepath.FromSlash(relative))
	if !pathWithinRoot(root, path) {
		return "", fmt.Errorf("folder path escapes the mod root: %q", relative)
	}
	return path, nil
}

// Returns the scanner's actual folder casing and permits the root only when it
// is used as a parent.
func knownFolder(folders []string, folder string) (string, error) {
	if folder == "" {
		return "", nil
	}

	for _, knownFolder := range folders {
		if strings.EqualFold(folder, knownFolder) {
			return knownFolder, nil
		}
	}
	return "", fmt.Errorf("folder is not present in the current scan: %q", folder)
}

// Requires a scanner-known folder that is safe to rename or move.
func mutableKnownFolder(folders []string, folder string) (string, error) {
	if folder == "" {
		return "", fmt.Errorf("mod root cannot be renamed or moved")
	}

	return knownFolder(folders, folder)
}

// Preserves the exact primary stem while relocating it to a known folder.
func movedStem(entry discovery.Entry, destinationFolder string) (string, error) {
	primarySuffix, err := primaryFileSuffix(entry.PrimaryPath)
	if err != nil {
		return "", err
	}

	stem := strings.TrimSuffix(filepath.Base(entry.PrimaryPath), primarySuffix)
	return filepath.ToSlash(filepath.Join(destinationFolder, stem)), nil
}

// Normalizes the root-level parent, which filepath.Dir represents as a dot.
func folderParent(folder string) string {
	parent := filepath.ToSlash(filepath.Dir(folder))
	if parent == "." {
		return ""
	}
	return parent
}

// Detects a destination below its source without relying on separator spelling.
func isDescendantFolder(folder, ancestor string) bool {
	relative, err := filepath.Rel(filepath.FromSlash(ancestor), filepath.FromSlash(folder))
	return err == nil && relative != "." && filepath.IsLocal(relative)
}

// Rejects links and special files because directory operations must not follow
// paths the scanner cannot model or reconcile reliably.
func requireDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing: %q", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a real directory: %q", label, path)
	}

	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode %s path: %w", label, err)
	}

	attributes, err := windows.GetFileAttributes(pathPointer)
	if err != nil {
		return fmt.Errorf("read %s attributes: %w", label, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%s is a reparse-point directory: %q", label, path)
	}
	return nil
}

// Validates each existing component from root through path immediately before
// mutation so a junction or symlink cannot redirect a lexical child path.
func requireDirectoryAncestry(root, path string, requireDirectoryFunc func(string, string) error) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || (relative != "." && !filepath.IsLocal(relative)) {
		return fmt.Errorf("directory ancestry escapes the mod root: %q", path)
	}

	if err := requireDirectoryFunc(root, "mod root"); err != nil {
		return err
	}

	current := root
	for _, component := range strings.FieldsFunc(filepath.ToSlash(relative), func(character rune) bool {
		return character == '/'
	}) {
		current = filepath.Join(current, component)
		if err := requireDirectoryFunc(current, "folder parent"); err != nil {
			return err
		}
	}
	return nil
}

// Reports the source and destination folder states after an unsuccessful move.
func reconcileFailedFolder(root, sourceFolder, destinationFolder string, cause error) (Result, error) {
	states := make([]string, 0, 2)
	for _, folder := range []string{sourceFolder, destinationFolder} {
		path, err := resolveFolderPath(root, folder)
		if err != nil {
			return Result{}, fmt.Errorf("%w; inspect affected folders: %v", cause, err)
		}

		_, err = os.Lstat(path)
		switch {
		case err == nil:
			states = append(states, fmt.Sprintf("%q exists", folder))
		case os.IsNotExist(err):
			states = append(states, fmt.Sprintf("%q is missing", folder))
		default:
			return Result{}, fmt.Errorf("%w; inspect affected folder %q: %v", cause, folder, err)
		}
	}
	return Result{}, fmt.Errorf("%w; affected folders: %s", cause, strings.Join(states, ", "))
}
