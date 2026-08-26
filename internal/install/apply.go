package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

const (
	// Restricted permissions for rollback backup storage directory.
	privateRollbackDirPermissions = 0700

	// Permissions for creating destination directories in modRoot.
	defaultDestinationDirPermissions = 0755

	// Maximum allowed UTF-16 code units for a mod name to comply with Windows filename limits.
	maximumModNameUTF16CodeUnits = 240

	// Lowest ASCII character code considered printable (characters 0 to 31 are control characters).
	minimumPrintableASCII = 32

	// Standard Marvel Rivals patch suffix attached to newly named mod primaries.
	applyPatchSuffix = "_P"
)

// ApplyItem configures how one staged mod is installed into the library.
type ApplyItem struct {
	ID                string `json:"id"`
	ModName           string `json:"modName"`
	DestinationFolder string `json:"destinationFolder"`
	Overwrite         bool   `json:"overwrite"`
}

// ApplyResult reports the final reconciled library state and IDs of installed mods.
type ApplyResult struct {
	InstalledEntryIDs []string          `json:"installedEntryIDs"`
	ReconciledLibrary discovery.Library `json:"reconciledLibrary"`
}

// Represents one planned file transfer from staging to modRoot.
type plannedFileInstall struct {
	sourceAbs      string
	destinationAbs string
	destRelative   string
	isOverwrite    bool
}

// Apply installs selected staged mods into modRoot transactionally.
func Apply(
	ctx context.Context,
	modRoot string,
	session *StagedSession,
	items []ApplyItem,
	checker mutation.GameRunningChecker,
) (ApplyResult, error) {
	if session == nil {
		return ApplyResult{}, fmt.Errorf("staging session is nil")
	}
	if len(items) == 0 {
		return ApplyResult{}, fmt.Errorf("no mod items selected for installation")
	}

	// Safety: Block mutations while Marvel Rivals is open.
	if checker != nil {
		running, err := checker.IsGameRunning()
		if err != nil {
			return ApplyResult{}, fmt.Errorf("check whether Marvel Rivals is running: %w", err)
		}
		if running {
			return ApplyResult{}, mutation.ErrGameRunning
		}
	}

	rootAbs, err := filepath.Abs(modRoot)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("resolve mod root: %w", err)
	}

	stagedModMap := make(map[string]StagedMod, len(session.Mods))
	for _, m := range session.Mods {
		stagedModMap[m.ID] = m
	}

	var plannedMoves []plannedFileInstall
	destinations := make(map[string]struct{})

	for _, item := range items {
		stagedMod, exists := stagedModMap[item.ID]
		if !exists {
			return ApplyResult{}, fmt.Errorf("staged mod %q not found in session", item.ID)
		}

		if err := validateModName(item.ModName); err != nil {
			return ApplyResult{}, fmt.Errorf("invalid name %q: %w", item.ModName, err)
		}

		normFolder := filepath.ToSlash(filepath.Clean(filepath.FromSlash(item.DestinationFolder)))
		if normFolder == "." || normFolder == "/" {
			normFolder = ""
		}
		if normFolder != "" && (!filepath.IsLocal(normFolder) || strings.HasPrefix(normFolder, "..")) {
			return ApplyResult{}, fmt.Errorf("destination folder %q escapes mod root", item.DestinationFolder)
		}

		destDirAbs := filepath.Join(rootAbs, filepath.FromSlash(normFolder))
		if !pathWithinRoot(rootAbs, destDirAbs) {
			return ApplyResult{}, fmt.Errorf("destination folder %q escapes mod root", item.DestinationFolder)
		}

		stem := determineFinalStem(stagedMod, item.ModName)

		// Primary file move
		primarySrcAbs := filepath.Join(session.Dir, filepath.FromSlash(stagedMod.RelativePrimaryPath))
		primaryDestRel := filepath.ToSlash(filepath.Join(normFolder, stem+".pak"))
		primaryDestAbs := filepath.Join(rootAbs, filepath.FromSlash(primaryDestRel))

		move, err := planFileMove(primarySrcAbs, primaryDestAbs, primaryDestRel, item.Overwrite, destinations)
		if err != nil {
			return ApplyResult{}, err
		}
		plannedMoves = append(plannedMoves, move)

		// Sidecars moves
		if stagedMod.Sidecars.UTOC != "" {
			utocSrcAbs := filepath.Join(session.Dir, filepath.FromSlash(stagedMod.Sidecars.UTOC))
			utocDestRel := filepath.ToSlash(filepath.Join(normFolder, stem+".utoc"))
			utocDestAbs := filepath.Join(rootAbs, filepath.FromSlash(utocDestRel))

			move, err := planFileMove(utocSrcAbs, utocDestAbs, utocDestRel, item.Overwrite, destinations)
			if err != nil {
				return ApplyResult{}, err
			}
			plannedMoves = append(plannedMoves, move)
		}

		if stagedMod.Sidecars.UCAS != "" {
			ucasSrcAbs := filepath.Join(session.Dir, filepath.FromSlash(stagedMod.Sidecars.UCAS))
			ucasDestRel := filepath.ToSlash(filepath.Join(normFolder, stem+".ucas"))
			ucasDestAbs := filepath.Join(rootAbs, filepath.FromSlash(ucasDestRel))

			move, err := planFileMove(ucasSrcAbs, ucasDestAbs, ucasDestRel, item.Overwrite, destinations)
			if err != nil {
				return ApplyResult{}, err
			}
			plannedMoves = append(plannedMoves, move)
		}
	}

	// Prepare rollback directory in staging
	rollbackDir := filepath.Join(session.Dir, "rollback")
	if err := os.MkdirAll(rollbackDir, privateRollbackDirPermissions); err != nil {
		return ApplyResult{}, fmt.Errorf("create rollback directory: %w", err)
	}

	var createdFiles []string
	var createdDirs []string
	overwrittenBackups := make(map[string]string) // destinationAbs -> backupAbs

	// Execute file moves with rollback capability
	var executionErr error
applyLoop:
	for i, move := range plannedMoves {
		select {
		case <-ctx.Done():
			executionErr = ctx.Err()
			break applyLoop
		default:
		}

		destDir := filepath.Dir(move.destinationAbs)

		// Discover which directories don't exist yet to track them for rollback
		var newDirs []string
		curr := destDir
		for {
			if _, err := os.Stat(curr); err == nil {
				break
			}
			newDirs = append(newDirs, curr)
			parent := filepath.Dir(curr)
			if parent == curr || parent == rootAbs {
				break
			}
			curr = parent
		}
		// Record highest-level directories last so they are removed last
		for j := len(newDirs) - 1; j >= 0; j-- {
			createdDirs = append(createdDirs, newDirs[j])
		}

		if err := os.MkdirAll(destDir, defaultDestinationDirPermissions); err != nil {
			executionErr = fmt.Errorf("create destination directory for %q: %w", move.destRelative, err)
			break applyLoop
		}

		if move.isOverwrite {
			// Backup destination file before replacing
			backupFile := filepath.Join(rollbackDir, fmt.Sprintf("backup_%d_%s", i, filepath.Base(move.destinationAbs)))
			if err := copyRegularFile(move.destinationAbs, backupFile); err != nil {
				executionErr = fmt.Errorf("backup existing file %q for rollback: %w", move.destRelative, err)
				break applyLoop
			}
			overwrittenBackups[move.destinationAbs] = backupFile
		}

		if !move.isOverwrite {
			createdFiles = append(createdFiles, move.destinationAbs)
		}

		if err := copyRegularFile(move.sourceAbs, move.destinationAbs); err != nil {
			executionErr = fmt.Errorf("install file to %q: %w", move.destRelative, err)
			break applyLoop
		}
	}

	if executionErr != nil {
		var rollbackErrs []string
		// Transaction failed - Rollback changes
		for _, createdPath := range createdFiles {
			if err := os.Remove(createdPath); err != nil && !os.IsNotExist(err) {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("remove newly created %q: %v", filepath.Base(createdPath), err))
			}
		}
		for destAbs, backupAbs := range overwrittenBackups {
			if err := copyRegularFile(backupAbs, destAbs); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("restore backup %q: %v", filepath.Base(destAbs), err))
			}
		}

		for i := len(createdDirs) - 1; i >= 0; i-- {
			// Best-effort: silently ignore errors (directory may be non-empty or already gone)
			os.Remove(createdDirs[i])
		}

		// Reconcile and report
		lib, _ := discovery.Scan(modRoot)
		if len(rollbackErrs) > 0 {
			return ApplyResult{ReconciledLibrary: lib}, fmt.Errorf("installation failed: %w; rollback encountered errors: %s", executionErr, strings.Join(rollbackErrs, "; "))
		}
		return ApplyResult{ReconciledLibrary: lib}, fmt.Errorf("installation failed partway and was cleanly rolled back: %w", executionErr)
	}

	// Rescan library to reconcile
	reconciledLib, err := discovery.Scan(modRoot)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("scan mod library after installation: %w", err)
	}

	var installedIDs []string
	for _, move := range plannedMoves {
		if strings.HasSuffix(strings.ToLower(move.destRelative), ".pak") {
			for _, entry := range reconciledLib.Entries {
				if strings.EqualFold(entry.PrimaryPath, move.destRelative) {
					installedIDs = append(installedIDs, entry.ID)
					break
				}
			}
		}
	}

	return ApplyResult{
		InstalledEntryIDs: installedIDs,
		ReconciledLibrary: reconciledLib,
	}, nil
}

// Determines the destination filename stem preserving priority if name unchanged.
func determineFinalStem(stagedMod StagedMod, requestedName string) string {
	if strings.EqualFold(requestedName, stagedMod.DisplayName) {
		// Keep original stem
		return stagedMod.Stem
	}

	// User changed the name: ensure clean name and standard _P patch suffix
	cleanName := strings.TrimSpace(requestedName)
	if strings.HasSuffix(strings.ToUpper(cleanName), applyPatchSuffix) {
		return cleanName
	}
	return cleanName + applyPatchSuffix
}

// Validates one planned file move before execution.
func planFileMove(srcAbs, destAbs, destRel string, overwrite bool, seenDests map[string]struct{}) (plannedFileInstall, error) {
	key := strings.ToLower(destAbs)
	if _, exists := seenDests[key]; exists {
		return plannedFileInstall{}, fmt.Errorf("multiple files map to destination %q", destRel)
	}
	seenDests[key] = struct{}{}

	if err := requireRegularFile(srcAbs, "staged source file"); err != nil {
		return plannedFileInstall{}, err
	}

	isOverwrite := false
	if _, err := os.Lstat(destAbs); err == nil {
		if !overwrite {
			return plannedFileInstall{}, fmt.Errorf("destination file %q already exists", destRel)
		}
		isOverwrite = true
	} else if !os.IsNotExist(err) {
		return plannedFileInstall{}, fmt.Errorf("check destination file %q: %w", destRel, err)
	}

	return plannedFileInstall{
		sourceAbs:      srcAbs,
		destinationAbs: destAbs,
		destRelative:   destRel,
		isOverwrite:    isOverwrite,
	}, nil
}

// Validates a user-entered mod name against Windows filename limits and reserved characters.
func validateModName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if !utf8.ValidString(trimmed) {
		return fmt.Errorf("name is not valid UTF-8")
	}
	if strings.HasSuffix(trimmed, " ") || strings.HasSuffix(trimmed, ".") {
		return fmt.Errorf("name cannot end with a space or period")
	}
	if strings.ContainsAny(trimmed, `<>:"/\|?*`) {
		return fmt.Errorf("name contains a Windows-reserved character")
	}
	if mutation.IsReservedDeviceName(trimmed) {
		return fmt.Errorf("name is reserved by Windows for a device")
	}
	if len(utf16.Encode([]rune(trimmed))) > maximumModNameUTF16CodeUnits {
		return fmt.Errorf("name is too long")
	}
	for _, character := range trimmed {
		if character < minimumPrintableASCII {
			return fmt.Errorf("name contains a control character")
		}
	}
	return nil
}

// Validates that path remains inside root.
func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(relative) && !strings.HasPrefix(relative, "..")
}

// Ensures the file exists and is a regular file.
func requireRegularFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
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
