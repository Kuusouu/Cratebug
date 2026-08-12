package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

const (
	// Windows filename components cannot exceed this many UTF-16 code units.
	maximumFileNameUTF16CodeUnits = 255
)

// Represents one planned no-replace rename with absolute endpoints and a
// root-relative destination suitable for actionable errors.
type fileMove struct {
	source              string
	sourceRelative      string
	destination         string
	destinationRelative string
}

// Groups every file move that must succeed for a controlled bundle rename to
// be considered successful. The primary paths are retained for reconciliation.
type bundlePlan struct {
	moves              []fileMove
	previousPrimary    string
	destinationPrimary string
}

// Names Windows reserves for devices rather than ordinary files.
type reservedDeviceName string

const (
	reservedDeviceConsole   reservedDeviceName = "CON"
	reservedDevicePrinter   reservedDeviceName = "PRN"
	reservedDeviceAuxiliary reservedDeviceName = "AUX"
	reservedDeviceNull      reservedDeviceName = "NUL"
	reservedDeviceCOM1      reservedDeviceName = "COM1"
	reservedDeviceCOM2      reservedDeviceName = "COM2"
	reservedDeviceCOM3      reservedDeviceName = "COM3"
	reservedDeviceCOM4      reservedDeviceName = "COM4"
	reservedDeviceCOM5      reservedDeviceName = "COM5"
	reservedDeviceCOM6      reservedDeviceName = "COM6"
	reservedDeviceCOM7      reservedDeviceName = "COM7"
	reservedDeviceCOM8      reservedDeviceName = "COM8"
	reservedDeviceCOM9      reservedDeviceName = "COM9"
	reservedDeviceLPT1      reservedDeviceName = "LPT1"
	reservedDeviceLPT2      reservedDeviceName = "LPT2"
	reservedDeviceLPT3      reservedDeviceName = "LPT3"
	reservedDeviceLPT4      reservedDeviceName = "LPT4"
	reservedDeviceLPT5      reservedDeviceName = "LPT5"
	reservedDeviceLPT6      reservedDeviceName = "LPT6"
	reservedDeviceLPT7      reservedDeviceName = "LPT7"
	reservedDeviceLPT8      reservedDeviceName = "LPT8"
	reservedDeviceLPT9      reservedDeviceName = "LPT9"
)

// Renames one current scanner entry while preserving its priority convention.
func renameMod(modRoot, entryID, name string) (Result, error) {
	return renameModWithMove(modRoot, entryID, name, moveFileWithoutReplace)
}

// Allows filesystem-failure tests to inject the rename primitive while keeping
// production behavior on the Windows no-replace implementation.
func renameModWithMove(modRoot, entryID, name string, move func(string, string) error) (Result, error) {
	// Rescanning prevents a stale scanner ID from selecting an arbitrary path.
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before rename: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Result{}, err
	}

	stem, err := renamedStem(entry, name)
	if err != nil {
		return Result{}, err
	}

	plan, err := buildBundlePlan(library.Root, entry, stem)
	if err != nil {
		return Result{}, err
	}
	if err := plan.apply(move); err != nil {
		return reconcileFailedPlan(modRoot, entry.ID, entry.PrimaryPath, plan, err)
	}

	return reconcileResult(modRoot, entry.ID, entry.PrimaryPath, plan.destinationPrimary)
}

// Changes the filename-based priority of one current scanner entry.
func setPriority(modRoot, entryID string, priority int) (Result, error) {
	return setPriorityWithMove(modRoot, entryID, priority, moveFileWithoutReplace)
}

// Allows priority failure tests to use the same transactional bundle flow as
// production code with an injected rename failure.
func setPriorityWithMove(modRoot, entryID string, priority int, move func(string, string) error) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before priority change: %w", err)
	}

	entry, err := findEntry(library.Entries, entryID)
	if err != nil {
		return Result{}, err
	}

	stem, err := priorityStem(entry, priority)
	if err != nil {
		return Result{}, err
	}

	plan, err := buildBundlePlan(library.Root, entry, stem)
	if err != nil {
		return Result{}, err
	}
	if err := plan.apply(move); err != nil {
		return reconcileFailedPlan(modRoot, entry.ID, entry.PrimaryPath, plan, err)
	}

	return reconcileResult(modRoot, entry.ID, entry.PrimaryPath, plan.destinationPrimary)
}

// Builds a complete, no-replace rename plan for the scanner-recognized bundle.
// Planning validates every source and destination before any file is moved.
func buildBundlePlan(modRoot string, entry discovery.Entry, destinationStem string) (bundlePlan, error) {
	if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
		return bundlePlan{}, fmt.Errorf("mod does not have a mutable primary file")
	}
	if hasIssue(entry.Issues, discovery.IssueAmbiguousPrimary) {
		return bundlePlan{}, fmt.Errorf("mod %q has multiple supported primaries and cannot be changed safely", entry.PrimaryPath)
	}
	if hasIssue(entry.Issues, discovery.IssueMissingUTOC) || hasIssue(entry.Issues, discovery.IssueMissingUCAS) {
		return bundlePlan{}, fmt.Errorf("mod %q is an incomplete IoStore bundle and cannot be changed safely", entry.PrimaryPath)
	}

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return bundlePlan{}, fmt.Errorf("resolve mod root: %w", err)
	}

	primarySuffix, err := primaryFileSuffix(entry.PrimaryPath)
	if err != nil {
		return bundlePlan{}, err
	}

	primaryDestination := destinationStem + primarySuffix
	paths := []struct {
		source      string
		destination string
		label       string
	}{
		{source: entry.PrimaryPath, destination: primaryDestination, label: "primary"},
	}
	for _, sidecar := range []string{entry.Sidecars.UTOC, entry.Sidecars.UCAS} {
		if sidecar == "" {
			continue
		}
		paths = append(paths, struct {
			source      string
			destination string
			label       string
		}{source: sidecar, destination: destinationStem + filepath.Ext(sidecar), label: "sidecar"})
	}

	// Windows treats destination names case-insensitively, so duplicate targets
	// must be rejected before the first move instead of relying on move order.
	plan := bundlePlan{previousPrimary: entry.PrimaryPath, destinationPrimary: primaryDestination}
	destinations := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		move, err := buildFileMove(root, path.source, path.destination, path.label)
		if err != nil {
			return bundlePlan{}, err
		}
		key := strings.ToLower(move.destination)
		if _, exists := destinations[key]; exists {
			return bundlePlan{}, fmt.Errorf("bundle rename maps multiple files to %q", move.destinationRelative)
		}
		destinations[key] = struct{}{}
		plan.moves = append(plan.moves, move)
	}
	return plan, nil
}

// Validates one bundle member before it becomes part of a multi-file rename.
// Both paths must be local relative paths even though scanner output is trusted,
// which keeps this boundary safe if a later caller changes its input source.
func buildFileMove(root, sourceRelative, destinationRelative, label string) (fileMove, error) {
	if !filepath.IsLocal(filepath.FromSlash(sourceRelative)) || !filepath.IsLocal(filepath.FromSlash(destinationRelative)) {
		return fileMove{}, fmt.Errorf("%s path is outside the mod root", label)
	}
	if len(utf16.Encode([]rune(filepath.Base(destinationRelative)))) > maximumFileNameUTF16CodeUnits {
		return fileMove{}, fmt.Errorf("destination %s filename is too long: %q", label, destinationRelative)
	}

	source := filepath.Join(root, filepath.FromSlash(sourceRelative))
	destination := filepath.Join(root, filepath.FromSlash(destinationRelative))
	if !pathWithinRoot(root, source) || !pathWithinRoot(root, destination) {
		return fileMove{}, fmt.Errorf("%s rename escapes the mod root", label)
	}
	if strings.EqualFold(source, destination) {
		return fileMove{}, fmt.Errorf("%s destination is unchanged: %q", label, sourceRelative)
	}
	if err := requireRegularFile(source, "source "+label); err != nil {
		return fileMove{}, err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fileMove{}, fmt.Errorf("destination %s already exists: %q", label, destinationRelative)
	} else if !os.IsNotExist(err) {
		return fileMove{}, fmt.Errorf("check destination %s: %w", label, err)
	}

	return fileMove{
		source:              source,
		sourceRelative:      sourceRelative,
		destination:         destination,
		destinationRelative: destinationRelative,
	}, nil
}

// Applies the complete plan and attempts rollback if a later move fails.
// Endpoint checks are repeated here because another process can change disk
// state after planning and before the first native rename.
func (plan bundlePlan) apply(move func(string, string) error) error {
	completed := make([]fileMove, 0, len(plan.moves))
	for _, item := range plan.moves {
		if err := requireRegularFile(item.source, "source bundle file"); err != nil {
			return err
		}
		if _, err := os.Lstat(item.destination); err == nil {
			return fmt.Errorf("destination bundle file already exists: %q", item.destinationRelative)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check destination bundle file: %w", err)
		}
		if err := move(item.source, item.destination); err != nil {
			rollbackErr := rollbackMoves(completed, move)
			if rollbackErr != nil {
				return fmt.Errorf("rename bundle file to %q: %w; rollback failed: %v", item.destinationRelative, err, rollbackErr)
			}
			return fmt.Errorf("rename bundle file to %q: %w", item.destinationRelative, err)
		}
		completed = append(completed, item)
	}

	for _, item := range plan.moves {
		if err := requireRegularFile(item.destination, "renamed bundle file"); err != nil {
			return fmt.Errorf("bundle rename completed but reconciliation failed: %w", err)
		}
		if _, err := os.Lstat(item.source); err == nil {
			return fmt.Errorf("bundle rename completed but source still exists: %q", item.source)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("reconcile source bundle file: %w", err)
		}
	}
	return nil
}

// Restores completed moves in reverse order so every rollback destination is
// vacant when its original source is restored.
func rollbackMoves(completed []fileMove, move func(string, string) error) error {
	for index := len(completed) - 1; index >= 0; index-- {
		item := completed[index]
		if err := move(item.destination, item.source); err != nil {
			return fmt.Errorf("restore %q: %w", item.source, err)
		}
	}
	return nil
}

// Reconciles a failed plan so callers receive the current primary when it can
// still be identified, while the error reports every affected path's final state.
func reconcileFailedPlan(modRoot, previousID, previousPrimary string, plan bundlePlan, cause error) (Result, error) {
	library, scanErr := discovery.Scan(modRoot)
	states, statesErr := plan.finalState()
	if scanErr != nil {
		return Result{}, fmt.Errorf("%w; reconcile mod library after failed mutation: %v; affected paths: %s", cause, scanErr, states)
	}
	if statesErr != nil {
		return Result{}, fmt.Errorf("%w; inspect affected paths after failed mutation: %v", cause, statesErr)
	}

	for _, primaryPath := range []string{plan.destinationPrimary, previousPrimary} {
		entry, err := findEntryByPrimaryPath(library.Entries, primaryPath)
		if err == nil {
			return Result{
				ID:                  entry.ID,
				PreviousID:          previousID,
				PreviousPrimaryPath: previousPrimary,
				PrimaryPath:         entry.PrimaryPath,
				State:               entry.State,
			}, fmt.Errorf("%w; affected paths: %s", cause, states)
		}
	}
	return Result{}, fmt.Errorf("%w; affected paths: %s", cause, states)
}

// Inspects every planned source and destination after a failed mutation instead
// of assuming rollback restored the original bundle.
func (plan bundlePlan) finalState() (string, error) {
	states := make([]string, 0, len(plan.moves)*2)
	for _, item := range plan.moves {
		for _, path := range []struct {
			absolute string
			relative string
		}{
			{absolute: item.source, relative: item.sourceRelative},
			{absolute: item.destination, relative: item.destinationRelative},
		} {
			_, err := os.Lstat(path.absolute)
			switch {
			case err == nil:
				states = append(states, fmt.Sprintf("%q exists", path.relative))
			case os.IsNotExist(err):
				states = append(states, fmt.Sprintf("%q is missing", path.relative))
			default:
				return "", fmt.Errorf("inspect %q: %w", path.relative, err)
			}
		}
	}
	return strings.Join(states, ", "), nil
}

// Rescans after a successful rename because scanner IDs incorporate the
// filename. The old ID lets a future UI replace only the affected entry.
func reconcileResult(modRoot, previousID, previousPrimary, primaryPath string) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("reconcile mod library after mutation: %w", err)
	}
	entry, err := findEntryByPrimaryPath(library.Entries, primaryPath)
	if err != nil {
		return Result{}, fmt.Errorf("reconcile renamed mod: %w", err)
	}

	return Result{
		ID:                  entry.ID,
		PreviousID:          previousID,
		PreviousPrimaryPath: previousPrimary,
		PrimaryPath:         entry.PrimaryPath,
		State:               entry.State,
	}, nil
}

// Finds the renamed entry by its planned path instead of assuming its scanner
// identity survives a filename change.
func findEntryByPrimaryPath(entries []discovery.Entry, primaryPath string) (discovery.Entry, error) {
	for _, entry := range entries {
		if entry.PrimaryPath == primaryPath {
			return entry, nil
		}
	}
	return discovery.Entry{}, fmt.Errorf("renamed mod is not present in the current scan: %q", primaryPath)
}

// Produces a new relative stem while retaining the mod's existing priority
// representation. Leading bangs must remain prefixes; trailing nines remain
// suffixes.
func renamedStem(entry discovery.Entry, name string) (string, error) {
	if err := validateFileStem(name); err != nil {
		return "", err
	}
	if entry.Priority.Kind == discovery.PriorityLeadingBang {
		name = "!" + name
	} else {
		name += priorityDecoration(entry)
	}
	return filepath.ToSlash(filepath.Join(entry.RelativeFolder, name)), nil
}

// Produces a Marvel Rivals priority filename. Priority zero uses the
// leading-bang form; positive values use seven or more trailing nines.
func priorityStem(entry discovery.Entry, priority int) (string, error) {
	if priority < 0 {
		return "", fmt.Errorf("priority must be zero or greater")
	}
	if priority > maximumFileNameUTF16CodeUnits {
		return "", fmt.Errorf("priority is too large for a valid filename")
	}

	name := entry.DisplayName
	if err := validateFileStem(name); err != nil {
		return "", fmt.Errorf("current mod name cannot receive a priority: %w", err)
	}
	if priority == 0 {
		return filepath.ToSlash(filepath.Join(entry.RelativeFolder, "!"+name)), nil
	}

	trailingNines := priority + discovery.MinimumTrailingNines - 1
	stem := name + "_" + strings.Repeat("9", trailingNines) + "_P"
	if err := validateFileStem(stem); err != nil {
		return "", fmt.Errorf("priority produces an invalid filename: %w", err)
	}
	return filepath.ToSlash(filepath.Join(entry.RelativeFolder, stem)), nil
}

// Returns the existing trailing priority suffix without changing the display
// name. The leading-bang form is handled separately because it is a prefix.
func priorityDecoration(entry discovery.Entry) string {
	switch entry.Priority.Kind {
	case discovery.PriorityLeadingBang:
		return "!"
	case discovery.PriorityTrailingNine:
		return entry.Priority.Raw[len(entry.DisplayName):]
	default:
		return ""
	}
}

// Preserves the primary's enabled or disabled format while its filename stem
// changes. Sidecars always use their own fixed extensions.
func primaryFileSuffix(path string) (string, error) {
	for _, suffix := range []string{crateoffSuffix, string(discovery.DisabledFormatBento), string(discovery.DisabledFormatLegacy), ".pak"} {
		if _, ok := trimSuffixFold(path, suffix); ok {
			return path[len(path)-len(suffix):], nil
		}
	}
	return "", fmt.Errorf("unsupported primary file format: %q", path)
}

// Rejects values that cannot become a single Windows filename component. This
// deliberately validates only the user-provided stem; folder safety belongs to
// bundle-plan construction.
func validateFileStem(stem string) error {
	if strings.TrimSpace(stem) == "" {
		return fmt.Errorf("mod name cannot be empty")
	}
	if !utf8.ValidString(stem) {
		return fmt.Errorf("mod name is not valid UTF-8")
	}
	if strings.HasSuffix(stem, " ") || strings.HasSuffix(stem, ".") {
		return fmt.Errorf("mod name cannot end with a space or period")
	}
	if strings.ContainsAny(stem, `<>:"/\|?*`) {
		return fmt.Errorf("mod name contains a Windows-reserved character")
	}
	if isReservedDeviceName(stem) {
		return fmt.Errorf("mod name is reserved by Windows for a device")
	}
	for _, character := range stem {
		if character < 32 {
			return fmt.Errorf("mod name contains a control character")
		}
	}
	return nil
}

// Windows reserves device names even when they are followed by an extension.
func isReservedDeviceName(stem string) bool {
	baseName := strings.TrimRight(stem, " .")
	if extension := strings.IndexRune(baseName, '.'); extension >= 0 {
		baseName = strings.TrimRight(baseName[:extension], " ")
	}
	switch reservedDeviceName(strings.ToUpper(baseName)) {
	case reservedDeviceConsole, reservedDevicePrinter, reservedDeviceAuxiliary, reservedDeviceNull,
		reservedDeviceCOM1, reservedDeviceCOM2, reservedDeviceCOM3, reservedDeviceCOM4,
		reservedDeviceCOM5, reservedDeviceCOM6, reservedDeviceCOM7, reservedDeviceCOM8,
		reservedDeviceCOM9, reservedDeviceLPT1, reservedDeviceLPT2, reservedDeviceLPT3,
		reservedDeviceLPT4, reservedDeviceLPT5, reservedDeviceLPT6, reservedDeviceLPT7,
		reservedDeviceLPT8, reservedDeviceLPT9:
		return true
	default:
		return false
	}
}
