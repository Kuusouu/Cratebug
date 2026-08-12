// Package mutation applies narrowly-scoped, validated changes to discovered mods.
package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"golang.org/x/sys/windows"
)

const crateoffSuffix = ".pak_crateoff"

// Reports the reconciled primary path and state after a successful mutation.
// Paths are relative to the mod root so callers never need filesystem paths from Go.
type Result struct {
	ID                  string          `json:"id"`
	PreviousPrimaryPath string          `json:"previousPrimaryPath"`
	PrimaryPath         string          `json:"primaryPath"`
	State               discovery.State `json:"state"`
}

// Contains the exact one-file rename validated before it is applied.
type plan struct {
	source              string
	destination         string
	destinationRelative string
	resultState         discovery.State
}

// Changes one scanner-discovered primary between enabled and disabled.
// It only renames the primary file; IoStore sidecars deliberately retain their names.
func setEnabled(modRoot, entryID string, enabled bool) (Result, error) {
	// Rescan so a stale frontend entry cannot select an arbitrary filesystem path.
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before mutation: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Result{}, err
	}

	if enabled == (entry.State == discovery.StateEnabled) {
		return Result{}, fmt.Errorf("mod %q is already %s", entry.PrimaryPath, entry.State)
	}

	// Planning performs every rejection-prone check before changing the primary.
	plan, err := buildPlan(library.Root, entry, enabled)
	if err != nil {
		return Result{}, err
	}

	if err := plan.apply(); err != nil {
		return Result{}, err
	}

	return Result{
		ID:                  entry.ID,
		PreviousPrimaryPath: entry.PrimaryPath,
		PrimaryPath:         plan.destinationRelative,
		State:               plan.resultState,
	}, nil
}

// Finds an entry by the scanner-issued identity returned to the frontend.
func findEntry(entries []discovery.Entry, entryID string) (discovery.Entry, error) {
	for _, entry := range entries {
		if entry.ID == entryID {
			return entry, nil
		}
	}
	return discovery.Entry{}, fmt.Errorf("mod is not present in the current scan: %q", entryID)
}

// Builds every filesystem path before mutation so rejected operations do not alter files.
func buildPlan(modRoot string, entry discovery.Entry, enabled bool) (plan, error) {
	// Only a scanner-produced mod entry can become a filesystem mutation target.
	if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
		return plan{}, fmt.Errorf("mod does not have a mutable primary file")
	}

	if hasIssue(entry.Issues, discovery.IssueAmbiguousPrimary) {
		return plan{}, fmt.Errorf("mod %q has multiple supported primaries and cannot be changed safely", entry.PrimaryPath)
	}

	if !filepath.IsLocal(filepath.FromSlash(entry.PrimaryPath)) {
		return plan{}, fmt.Errorf("mod primary path is outside the mod root: %q", entry.PrimaryPath)
	}

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return plan{}, fmt.Errorf("resolve mod root: %w", err)
	}

	source := filepath.Join(root, filepath.FromSlash(entry.PrimaryPath))
	destinationRelative, err := transitionPath(entry.PrimaryPath, entry.State, enabled)
	if err != nil {
		return plan{}, err
	}

	destination := filepath.Join(root, filepath.FromSlash(destinationRelative))
	if !pathWithinRoot(root, source) || !pathWithinRoot(root, destination) {
		return plan{}, fmt.Errorf("mod transition escapes the mod root")
	}

	// Validate both endpoint paths before the Windows no-replace rename.
	if err := requireRegularFile(source, "source primary"); err != nil {
		return plan{}, err
	}

	if _, err := os.Lstat(destination); err == nil {
		return plan{}, fmt.Errorf("destination primary already exists: %q", destinationRelative)
	} else if !os.IsNotExist(err) {
		return plan{}, fmt.Errorf("check destination primary: %w", err)
	}

	state := discovery.StateDisabled
	if enabled {
		state = discovery.StateEnabled
	}
	return plan{source: source, destination: destination, destinationRelative: destinationRelative, resultState: state}, nil
}

// Applies the single-file rename after the complete plan has passed validation.
func (p plan) apply() error {
	// Repeat endpoint checks to close the gap between planning and mutation.
	if err := requireRegularFile(p.source, "source primary"); err != nil {
		return err
	}

	if _, err := os.Lstat(p.destination); err == nil {
		return fmt.Errorf("destination primary already exists: %q", p.destinationRelative)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check destination primary: %w", err)
	}

	if err := moveFileWithoutReplace(p.source, p.destination); err != nil {
		return fmt.Errorf("rename primary to %q: %w", p.destinationRelative, err)
	}

	// Reconciliation prevents a false success if the filesystem reports an unexpected final state.
	if err := requireRegularFile(p.destination, "renamed primary"); err != nil {
		return fmt.Errorf("primary rename completed but reconciliation failed: %w", err)
	}

	if _, err := os.Lstat(p.source); err == nil {
		return fmt.Errorf("primary rename completed but source still exists: %q", p.source)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reconcile source primary: %w", err)
	}
	return nil
}

// moveFileWithoutReplace uses MoveFileW because it fails if the destination exists.
// os.Rename does not provide that no-clobber guarantee on Windows.
func moveFileWithoutReplace(source, destination string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encode source path: %w", err)
	}

	destinationPointer, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return fmt.Errorf("encode destination path: %w", err)
	}

	return windows.MoveFile(sourcePointer, destinationPointer)
}

// Maps the scanner's supported filename formats to the requested state.
func transitionPath(primaryPath string, state discovery.State, enabled bool) (string, error) {
	if enabled {
		// Enabling accepts every disabled convention we promise to read.
		if state != discovery.StateDisabled {
			return "", fmt.Errorf("cannot enable primary in state %q", state)
		}

		for _, suffix := range []string{crateoffSuffix, string(discovery.DisabledFormatBento), string(discovery.DisabledFormatLegacy)} {
			if stem, ok := trimSuffixFold(primaryPath, suffix); ok {
				return stem + ".pak", nil
			}
		}
		return "", fmt.Errorf("unsupported disabled primary format: %q", primaryPath)
	}

	if state != discovery.StateEnabled {
		return "", fmt.Errorf("cannot disable primary in state %q", state)
	}

	if stem, ok := trimSuffixFold(primaryPath, ".pak"); ok {
		return stem + crateoffSuffix, nil
	}
	return "", fmt.Errorf("enabled primary does not have a .pak suffix: %q", primaryPath)
}

// Rejects path traversal even if a future scanner field is accidentally used as input.
func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(relative)
}

// Requires regular files because renaming links or special files would bypass the scanner's model.
func requireRegularFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		// A missing source is actionable, while other stat errors retain their system detail.
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing: %q", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file: %q", label, path)
	}
	return nil
}

// Keeps matching aligned with discovery's case-insensitive filename classification.
func trimSuffixFold(value, suffix string) (string, bool) {
	if len(value) < len(suffix) || !strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return "", false
	}
	return value[:len(value)-len(suffix)], true
}

// Avoids exposing mutation policy through discovery while still honoring scanner diagnostics.
func hasIssue(issues []discovery.Issue, code discovery.IssueCode) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
