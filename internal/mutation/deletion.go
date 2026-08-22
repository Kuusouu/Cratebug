package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

type bundleFile struct {
	absolute string
	relative string
}

type deletionPlan struct {
	files           []bundleFile
	previousID      string
	previousPrimary string
}

// Deletes one scanner-discovered bundle through the Recycle Bin.
func deleteMod(modRoot, entryID string, confirmed bool) (Result, error) {
	return deleteModWithRecycle(modRoot, entryID, confirmed, recycleFiles)
}

// Allows disposable tests to substitute the Windows Recycle Bin boundary.
func deleteModWithRecycle(modRoot, entryID string, confirmed bool, recycle func([]string) error) (Result, error) {
	if !confirmed {
		return Result{}, fmt.Errorf("deletion requires explicit confirmation")
	}

	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before deletion: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Result{}, err
	}

	plan, err := buildDeletionPlan(library.Root, entry)
	if err != nil {
		return Result{}, err
	}
	if err := plan.apply(library.Root, recycle); err != nil {
		return reconcileFailedDeletion(modRoot, plan, err)
	}

	if err := reconcileDeletedBundle(modRoot, plan); err != nil {
		return Result{}, err
	}
	return Result{
		PreviousID:          plan.previousID,
		PreviousPrimaryPath: plan.previousPrimary,
		Deleted:             true,
	}, nil
}

// Rescans after the shell operation so the reported deletion state agrees with
// the same discovery model that identified the bundle.
func reconcileDeletedBundle(modRoot string, plan deletionPlan) error {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return fmt.Errorf("reconcile mod library after deletion: %w", err)
	}
	for _, entry := range library.Entries {
		if entry.PrimaryPath == plan.previousPrimary {
			return fmt.Errorf("Recycle Bin operation completed but deleted primary remains in the scan: %q", plan.previousPrimary)
		}
	}
	return nil
}

// Builds a deletion plan before calling the platform Recycle Bin. Incomplete
// IoStore bundles are allowed so their present recognized members can be
// recovered from the Recycle Bin together.
func buildDeletionPlan(modRoot string, entry discovery.Entry) (deletionPlan, error) {
	if err := validateDeletableBundleEntry(entry); err != nil {
		return deletionPlan{}, err
	}

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return deletionPlan{}, fmt.Errorf("resolve mod root: %w", err)
	}

	paths := []string{entry.PrimaryPath, entry.Sidecars.UTOC, entry.Sidecars.UCAS}
	plan := deletionPlan{previousID: entry.ID, previousPrimary: entry.PrimaryPath}
	for _, relative := range paths {
		if relative == "" {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(relative)) {
			return deletionPlan{}, fmt.Errorf("bundle path is outside the mod root: %q", relative)
		}

		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if !pathWithinRoot(root, absolute) {
			return deletionPlan{}, fmt.Errorf("bundle path escapes the mod root: %q", relative)
		}
		if err := requireRegularFile(absolute, "source bundle file"); err != nil {
			return deletionPlan{}, err
		}
		plan.files = append(plan.files, bundleFile{absolute: absolute, relative: relative})
	}

	if err := requireDeletionDirectoryAncestry(root, plan); err != nil {
		return deletionPlan{}, err
	}
	return plan, nil
}

// Rechecks files and their ancestors immediately before shell deletion.
func (plan deletionPlan) apply(root string, recycle func([]string) error) error {
	paths := make([]string, 0, len(plan.files))
	for _, file := range plan.files {
		if err := requireRegularFile(file.absolute, "source bundle file"); err != nil {
			return err
		}
		paths = append(paths, file.absolute)
	}
	if err := requireDeletionDirectoryAncestry(root, plan); err != nil {
		return err
	}

	if err := recycle(paths); err != nil {
		return fmt.Errorf("send bundle to Recycle Bin: %w", err)
	}

	for _, file := range plan.files {
		if _, err := os.Lstat(file.absolute); err == nil {
			return fmt.Errorf("Recycle Bin operation completed but bundle file remains: %q", file.relative)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("reconcile deleted bundle file: %w", err)
		}
	}
	return nil
}

func requireDeletionDirectoryAncestry(root string, plan deletionPlan) error {
	for _, file := range plan.files {
		if err := requireDirectoryAncestry(root, filepath.Dir(file.absolute), requireDirectory); err != nil {
			return err
		}
	}
	return nil
}

// Reports the remaining files after a failed shell deletion rather than
// assuming all bundle members were recycled together.
func reconcileFailedDeletion(modRoot string, plan deletionPlan, cause error) (Result, error) {
	states, err := plan.finalState()
	if err != nil {
		return Result{}, fmt.Errorf("%w; inspect affected bundle files: %v", cause, err)
	}

	library, scanErr := discovery.Scan(modRoot)
	if scanErr != nil {
		return Result{}, fmt.Errorf("%w; reconcile mod library after failed deletion: %v; affected paths: %s", cause, scanErr, states)
	}
	entry, entryErr := findEntryByPrimaryPath(library.Entries, plan.previousPrimary)
	if entryErr == nil {
		return Result{
			ID:                  entry.ID,
			PreviousID:          plan.previousID,
			PreviousPrimaryPath: plan.previousPrimary,
			PrimaryPath:         entry.PrimaryPath,
			State:               entry.State,
		}, fmt.Errorf("%w; affected paths: %s", cause, states)
	}
	return Result{}, fmt.Errorf("%w; affected paths: %s", cause, states)
}

func (plan deletionPlan) finalState() (string, error) {
	states := make([]string, 0, len(plan.files))
	for _, file := range plan.files {
		_, err := os.Lstat(file.absolute)
		switch {
		case err == nil:
			states = append(states, fmt.Sprintf("%q exists", file.relative))
		case os.IsNotExist(err):
			states = append(states, fmt.Sprintf("%q is missing", file.relative))
		default:
			return "", fmt.Errorf("inspect %q: %w", file.relative, err)
		}
	}
	return strings.Join(states, ", "), nil
}
