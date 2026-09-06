package mutation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Reported when the user cancels an in-progress companion PAK cleanup.
var ErrCompanionCleanupCancelled = errors.New("companion PAK cleanup cancelled")

// Names one member that failed inside a companion PAK cleanup batch.
type CompanionCleanupFailure struct {
	EntryID string `json:"entryID"`
	Message string `json:"message"`
}

// Reports which checked IDs were rewritten and which failed.
type CompanionCleanupResult struct {
	Succeeded []string                  `json:"succeeded"`
	Failed    []CompanionCleanupFailure `json:"failed"`
}

// Identifies the bundle currently being rewritten so the UI can show progress.
type CompanionCleanupProgress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	EntryID     string `json:"entryID"`
	DisplayName string `json:"displayName"`
}

type companionCaller interface {
	Call(action string, params map[string]any, result any) error
}

// Lists scanner IDs whose primary PAK still contains chunknames or patched_files.
// A listing error skips that member so one unreadable PAK cannot hide the rest.
func FindUnsupportedCompanionPaks(modRoot string, caller companionCaller) ([]string, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return nil, fmt.Errorf("scan mod library before companion PAK inspection: %w", err)
	}

	root, err := filepath.Abs(library.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve mod root: %w", err)
	}

	var dirty []string
	for _, entry := range library.Entries {
		if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
			continue
		}
		pakAbs := filepath.Join(root, filepath.FromSlash(entry.PrimaryPath))
		if !pathWithinRoot(root, pakAbs) {
			continue
		}
		utocAbs := ""
		if entry.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC))
		}
		listing, _, err := listCompanionPak(caller, pakAbs, utocAbs)
		if err != nil {
			continue
		}
		if len(uassettool.CompanionPakUnsupportedPaths(listing)) > 0 {
			dirty = append(dirty, entry.ID)
		}
	}
	return dirty, nil
}

// Rewrites each primary PAK in entryIDs that still contains unsupported
// companion metadata names. .utoc and .ucas are not opened for write.
func StripCompanionPaks(
	modRoot string,
	entryIDs []string,
	caller companionCaller,
	progress func(CompanionCleanupProgress),
	cancel <-chan struct{},
) (CompanionCleanupResult, error) {
	if len(entryIDs) == 0 {
		return CompanionCleanupResult{}, fmt.Errorf("no mods selected")
	}

	library, err := discovery.Scan(modRoot)
	if err != nil {
		return CompanionCleanupResult{}, fmt.Errorf("scan mod library before companion PAK cleanup: %w", err)
	}

	root, err := filepath.Abs(library.Root)
	if err != nil {
		return CompanionCleanupResult{}, fmt.Errorf("resolve mod root: %w", err)
	}

	targets := make([]discovery.Entry, 0, len(entryIDs))
	for _, id := range entryIDs {
		entry, err := findEntry(library.Entries, id)
		if err != nil {
			return CompanionCleanupResult{}, err
		}
		if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
			return CompanionCleanupResult{}, fmt.Errorf("entry %q is not a mod with a primary PAK", id)
		}
		targets = append(targets, entry)
	}

	result := CompanionCleanupResult{}
	for index, entry := range targets {
		if cancelled(cancel) {
			result.Failed = append(result.Failed, CompanionCleanupFailure{
				EntryID: entry.ID,
				Message: ErrCompanionCleanupCancelled.Error(),
			})
			return result, ErrCompanionCleanupCancelled
		}
		if progress != nil {
			progress(CompanionCleanupProgress{
				Current:     index + 1,
				Total:       len(targets),
				EntryID:     entry.ID,
				DisplayName: entry.DisplayName,
			})
		}
		pakAbs := filepath.Join(root, filepath.FromSlash(entry.PrimaryPath))
		utocAbs := ""
		if entry.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC))
		}
		if err := RewriteCompanionPak(root, pakAbs, utocAbs, caller); err != nil {
			result.Failed = append(result.Failed, CompanionCleanupFailure{
				EntryID: entry.ID,
				Message: err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, entry.ID)
	}
	return result, nil
}

// Reports whether pakAbs lists chunknames or patched_files. Listing errors
// are returned so install can fail closed instead of copying a dirty PAK.
func CompanionPakNeedsCleanup(pakAbs, utocAbs string, caller companionCaller) (bool, error) {
	listing, _, err := listCompanionPak(caller, pakAbs, utocAbs)
	if err != nil {
		return false, err
	}
	return len(uassettool.CompanionPakUnsupportedPaths(listing)) > 0, nil
}

// Rewrites pakAbs in place when its listing contains unsupported companion
// metadata names. root must contain pakAbs. utocAbs may be empty.
func RewriteCompanionPak(root, pakAbs, utocAbs string, caller companionCaller) error {
	if !pathWithinRoot(root, pakAbs) {
		return fmt.Errorf("companion PAK escapes the mod root")
	}
	if err := requireRegularFile(pakAbs, "companion PAK"); err != nil {
		return err
	}

	listing, _, err := listCompanionPak(caller, pakAbs, utocAbs)
	if err != nil {
		return fmt.Errorf("list companion PAK: %w", err)
	}
	if len(uassettool.CompanionPakUnsupportedPaths(listing)) == 0 {
		return nil
	}

	workDir, err := os.MkdirTemp("", "cratebug-companion-*")
	if err != nil {
		return fmt.Errorf("create companion work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	nextPak := filepath.Join(workDir, "rewritten.pak")
	filePaths, err := companionRewriteInputs(caller, pakAbs, listing, workDir)
	if err != nil {
		return err
	}
	if err := uassettool.CreatePak(caller, nextPak, filePaths, ""); err != nil {
		return fmt.Errorf("rebuild companion PAK: %w", err)
	}
	return replaceBundleFiles(root, map[string]string{pakAbs: nextPak})
}

func listCompanionPak(caller companionCaller, pakAbs, utocAbs string) ([]uassettool.PakEntry, string, error) {
	// An obfuscated IoStore can still list_pak without a key and return a
	// listing that hides chunknames. When the utoc is encrypted, the key
	// listing is the one that matters.
	if utocAbs != "" {
		encrypted, err := uassettool.IsIoStoreEncrypted(caller, utocAbs)
		if err == nil && encrypted {
			listing, err := uassettool.ListPakWithKey(caller, pakAbs, uassettool.MarvelRivalsAESKey)
			if err != nil {
				return nil, "", fmt.Errorf("list encrypted companion PAK: %w", err)
			}
			return listing, uassettool.MarvelRivalsAESKey, nil
		}
	}

	listing, err := uassettool.ListPakWithKey(caller, pakAbs, "")
	if err == nil {
		return listing, "", nil
	}
	listing, keyErr := uassettool.ListPakWithKey(caller, pakAbs, uassettool.MarvelRivalsAESKey)
	if keyErr == nil {
		return listing, uassettool.MarvelRivalsAESKey, nil
	}
	return nil, "", err
}

func companionRewriteInputs(caller companionCaller, pakAbs string, listing []uassettool.PakEntry, workDir string) ([]string, error) {
	if !uassettool.CompanionPakHasRawFiles(listing) {
		return companionStubFilePaths(workDir)
	}

	extractDir := filepath.Join(workDir, "extract")
	if err := os.Mkdir(extractDir, 0o700); err != nil {
		return nil, fmt.Errorf("create companion extract directory: %w", err)
	}
	if _, err := uassettool.ExtractPakAll(caller, pakAbs, extractDir, ""); err != nil {
		if _, err = uassettool.ExtractPakAll(caller, pakAbs, extractDir, uassettool.MarvelRivalsAESKey); err != nil {
			return nil, fmt.Errorf("extract companion PAK: %w", err)
		}
	}

	var filePaths []string
	for _, entry := range listing {
		if uassettool.IsCompanionMetadataPath(entry.Path) {
			continue
		}
		extracted, err := resolveExtractedPakFile(extractDir, entry.Path)
		if err != nil {
			return nil, err
		}
		filePaths = append(filePaths, entry.Path+"="+extracted)
	}
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("companion PAK has raw files that could not be extracted")
	}
	return filePaths, nil
}

// The pinned create_pak action rejects an empty file list, so a
// metadata-only companion needs one harmless placeholder instead.
func companionStubFilePaths(workDir string) ([]string, error) {
	path := filepath.Join(workDir, uassettool.CompanionStubName)
	if err := os.WriteFile(path, []byte("cratebug companion stub\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write companion stub file: %w", err)
	}
	return []string{uassettool.CompanionStubName + "=" + path}, nil
}

func resolveExtractedPakFile(extractDir, pakPath string) (string, error) {
	trimmed := strings.TrimLeft(filepath.ToSlash(pakPath), "/")
	candidates := []string{
		filepath.Join(extractDir, filepath.FromSlash(trimmed)),
		filepath.Join(extractDir, filepath.Base(pakPath)),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("extracted companion file missing for %q", pakPath)
}
