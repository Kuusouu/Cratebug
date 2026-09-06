package mutation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Reported when the checked set mixes encrypted and unencrypted IoStore mods.
var ErrEncryptionMixedState = errors.New("selected mods have a mixed encryption state")

// Reported when a checked mod is not a complete IoStore bundle.
var ErrEncryptionIneligible = errors.New("selected mods include a bundle that cannot be encrypted")

// Reported when the user cancels an in-progress encrypt or decrypt.
var ErrEncryptionCancelled = errors.New("encryption cancelled")

// Names one member that failed inside an encryption batch.
type EncryptionFailure struct {
	EntryID string `json:"entryID"`
	Message string `json:"message"`
}

// Reports which checked IDs were rewritten and which failed.
type EncryptionBatchResult struct {
	Succeeded []string            `json:"succeeded"`
	Failed    []EncryptionFailure `json:"failed"`
}

// Identifies the bundle currently being rebuilt so the UI can show progress.
type EncryptionProgress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	EntryID     string `json:"entryID"`
	DisplayName string `json:"displayName"`
}

type encryptionCaller interface {
	Call(action string, params map[string]any, result any) error
}

type bundleSwap struct {
	live string
	next string
	bak  string
}

// Rebuilds each complete IoStore bundle in entryIDs with or without
// obfuscation. Mixed encryption state and ineligible formats fail before
// any file is rewritten. encrypt true writes obfuscated output.
func SetModEncryption(
	modRoot string,
	entryIDs []string,
	encrypt bool,
	caller encryptionCaller,
	progress func(EncryptionProgress),
	cancel <-chan struct{},
) (EncryptionBatchResult, error) {
	if len(entryIDs) == 0 {
		return EncryptionBatchResult{}, fmt.Errorf("no mods selected")
	}

	library, err := discovery.Scan(modRoot)
	if err != nil {
		return EncryptionBatchResult{}, fmt.Errorf("scan mod library before encryption: %w", err)
	}

	root, err := filepath.Abs(library.Root)
	if err != nil {
		return EncryptionBatchResult{}, fmt.Errorf("resolve mod root: %w", err)
	}

	targets := make([]discovery.Entry, 0, len(entryIDs))
	var encryptedCount int
	for _, id := range entryIDs {
		entry, err := findEntry(library.Entries, id)
		if err != nil {
			return EncryptionBatchResult{}, err
		}
		if err := validateEncryptableEntry(entry); err != nil {
			return EncryptionBatchResult{}, err
		}

		utocPath := filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC))
		encrypted, err := uassettool.IsIoStoreEncrypted(caller, utocPath)
		if err != nil {
			return EncryptionBatchResult{}, fmt.Errorf("check encryption for %q: %w", entry.DisplayName, err)
		}
		if encrypted {
			encryptedCount++
		}
		targets = append(targets, entry)
	}

	if encryptedCount != 0 && encryptedCount != len(targets) {
		return EncryptionBatchResult{}, ErrEncryptionMixedState
	}
	alreadyEncrypted := encryptedCount == len(targets)
	if alreadyEncrypted == encrypt {
		return EncryptionBatchResult{Succeeded: append([]string(nil), entryIDs...)}, nil
	}

	result := EncryptionBatchResult{}
	for index, entry := range targets {
		if cancelled(cancel) {
			result.Failed = append(result.Failed, EncryptionFailure{EntryID: entry.ID, Message: ErrEncryptionCancelled.Error()})
			return result, ErrEncryptionCancelled
		}
		if progress != nil {
			progress(EncryptionProgress{
				Current:     index + 1,
				Total:       len(targets),
				EntryID:     entry.ID,
				DisplayName: entry.DisplayName,
			})
		}
		if err := rewriteBundleEncryption(root, entry, encrypt, alreadyEncrypted, caller); err != nil {
			result.Failed = append(result.Failed, EncryptionFailure{EntryID: entry.ID, Message: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, entry.ID)
	}
	return result, nil
}

func validateEncryptableEntry(entry discovery.Entry) error {
	if err := validateMutableBundleEntry(entry); err != nil {
		return fmt.Errorf("%w: %v", ErrEncryptionIneligible, err)
	}
	if entry.BundleFormat != discovery.BundleFormatIoStore {
		return fmt.Errorf("%w: %q is not an IoStore bundle", ErrEncryptionIneligible, entry.DisplayName)
	}
	if entry.Sidecars.UTOC == "" || entry.Sidecars.UCAS == "" {
		return fmt.Errorf("%w: %q is an incomplete IoStore bundle", ErrEncryptionIneligible, entry.DisplayName)
	}
	return nil
}

func rewriteBundleEncryption(root string, entry discovery.Entry, encrypt, currentlyEncrypted bool, caller encryptionCaller) error {
	primaryAbs := filepath.Join(root, filepath.FromSlash(entry.PrimaryPath))
	utocAbs := filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC))
	ucasAbs := filepath.Join(root, filepath.FromSlash(entry.Sidecars.UCAS))
	if !pathWithinRoot(root, primaryAbs) || !pathWithinRoot(root, utocAbs) || !pathWithinRoot(root, ucasAbs) {
		return fmt.Errorf("bundle paths escape the mod root")
	}

	workDir, err := os.MkdirTemp("", "cratebug-encrypt-*")
	if err != nil {
		return fmt.Errorf("create encryption work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	extractDir := filepath.Join(workDir, "extract")
	if err := os.Mkdir(extractDir, 0o700); err != nil {
		return fmt.Errorf("create extract directory: %w", err)
	}

	// Extract uses a key only when the live container is already encrypted.
	// create_mod_iostore treats aes_key as input-PAK read key, not output
	// encryption. Output encryption is obfuscate alone.
	extractKey := ""
	if currentlyEncrypted {
		extractKey = uassettool.MarvelRivalsAESKey
	}
	if _, err := uassettool.ExtractIoStore(caller, utocAbs, extractDir, extractKey); err != nil {
		return fmt.Errorf("extract IoStore: %w", err)
	}

	hybrid := false
	rawExtracted := 0
	pakEntries, err := uassettool.ListPakWithKey(caller, primaryAbs, extractKey)
	if err != nil {
		return fmt.Errorf("list companion PAK: %w", err)
	}
	if uassettool.CompanionPakHasRawFiles(pakEntries) {
		hybrid = true
		count, err := uassettool.ExtractPakAll(caller, primaryAbs, extractDir, extractKey)
		if err != nil {
			return fmt.Errorf("extract companion PAK: %w", err)
		}
		rawExtracted = count
	}

	outputBase := filepath.Join(workDir, "rebuilt")
	options := uassettool.IoStoreCreateOptions{
		Obfuscate: encrypt,
		Hybrid:    hybrid,
		Compress:  true,
	}
	inputDir := extractDir
	if hybrid && !extractDirHasUnrealFiles(extractDir) {
		// extract_pak_all drops leading-slash PAK paths on Windows. The
		// worker's input_pak extract trims them and keeps the raw files.
		inputDir = ""
		options.InputPak = primaryAbs
		options.AESKey = extractKey
	} else if hybrid && rawExtracted == 0 {
		return fmt.Errorf("companion PAK has raw files that could not be extracted")
	}
	if _, err := uassettool.CreateModIoStore(caller, outputBase, inputDir, options); err != nil {
		return fmt.Errorf("rebuild IoStore: %w", err)
	}

	rebuiltPak := outputBase + ".pak"
	rebuiltUtoc := outputBase + ".utoc"
	// create_mod_iostore can write chunknames / patched_files back into the
	// companion PAK. Strip them here so the library never receives those names.
	if err := RewriteCompanionPak(workDir, rebuiltPak, rebuiltUtoc, caller); err != nil {
		return fmt.Errorf("strip companion metadata after rebuild: %w", err)
	}

	rebuilt := map[string]string{
		primaryAbs: rebuiltPak,
		utocAbs:    rebuiltUtoc,
		ucasAbs:    outputBase + ".ucas",
	}
	return replaceBundleFiles(root, rebuilt)
}

func replaceBundleFiles(root string, replacements map[string]string) error {
	swaps := make([]bundleSwap, 0, len(replacements))
	for live, next := range replacements {
		if !pathWithinRoot(root, live) {
			return fmt.Errorf("replacement escapes the mod root")
		}
		if err := requireRegularFile(next, "rebuilt bundle file"); err != nil {
			return err
		}
		if err := requireRegularFile(live, "live bundle file"); err != nil {
			return err
		}
		swaps = append(swaps, bundleSwap{
			live: live,
			next: next,
			bak:  live + ".cratebug-encrypt-bak",
		})
	}

	applied := 0
	for _, item := range swaps {
		if err := os.Rename(item.live, item.bak); err != nil {
			_ = restoreEncryptionBackups(swaps[:applied])
			return fmt.Errorf("park live bundle file: %w", err)
		}
		if err := copyRegularFile(item.next, item.live); err != nil {
			_ = os.Rename(item.bak, item.live)
			_ = restoreEncryptionBackups(swaps[:applied])
			return fmt.Errorf("install rebuilt bundle file: %w", err)
		}
		applied++
	}

	for _, item := range swaps {
		_ = os.Remove(item.bak)
	}
	return nil
}

func restoreEncryptionBackups(swaps []bundleSwap) error {
	var first error
	for i := len(swaps) - 1; i >= 0; i-- {
		item := swaps[i]
		_ = os.Remove(item.live)
		if err := os.Rename(item.bak, item.live); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func copyRegularFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(destination)
		return err
	}
	return out.Close()
}

func extractDirHasUnrealFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".uasset") || strings.HasSuffix(name, ".ushaderbytecode") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func cancelled(cancel <-chan struct{}) bool {
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}
