package mutation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/modtype"
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

// Identifies the bundle whose encryption state changes so the UI can show progress.
type EncryptionProgress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	EntryID     string `json:"entryID"`
	DisplayName string `json:"displayName"`
}

// ArchiveCaller is the UAssetTool request surface used by encrypt.
type ArchiveCaller interface {
	Call(action string, params map[string]any, result any) error
}

// ArchiveCallerFactory starts one caller and returns cleanup that must run.
type ArchiveCallerFactory func() (caller ArchiveCaller, cleanup func(), err error)

type encryptionWorker struct {
	caller  ArchiveCaller
	cleanup func()
}

type bundleSwap struct {
	live string
	next string
	bak  string
}

// Encrypts or decrypts existing IoStore blocks without a legacy asset conversion.
// Mixed state and incomplete bundles fail before any file changes.
func SetModEncryption(
	modRoot string,
	entryIDs []string,
	encrypt bool,
	caller ArchiveCaller,
	progress func(EncryptionProgress),
	cancel <-chan struct{},
) (EncryptionBatchResult, error) {
	root, targets, early, done, err := planEncryption(modRoot, entryIDs, encrypt, caller)
	if err != nil {
		return EncryptionBatchResult{}, err
	}
	if done {
		return early, nil
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
		if err := rewriteBundleEncryption(root, entry, encrypt, caller); err != nil {
			result.Failed = append(result.Failed, EncryptionFailure{EntryID: entry.ID, Message: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, entry.ID)
	}
	return result, nil
}

// Same as SetModEncryption, but rewrites through one write worker per pool
// slot. Size follows DefaultWorkerPoolSizeForLibrary. Mixed state and
// ineligible formats still fail before any rewrite. launch must return a
// caller that is safe to use from only one goroutine.
func SetModEncryptionPooled(
	modRoot string,
	entryIDs []string,
	encrypt bool,
	launch ArchiveCallerFactory,
	progress func(EncryptionProgress),
	cancel <-chan struct{},
) (EncryptionBatchResult, error) {
	if launch == nil {
		return EncryptionBatchResult{}, fmt.Errorf("encryption worker factory is required")
	}
	first, cleanupFirst, err := launch()
	if err != nil {
		return EncryptionBatchResult{}, fmt.Errorf("start encryption worker: %w", err)
	}

	root, targets, early, done, err := planEncryption(modRoot, entryIDs, encrypt, first)
	if err != nil {
		if cleanupFirst != nil {
			cleanupFirst()
		}
		return EncryptionBatchResult{}, err
	}
	if done {
		if cleanupFirst != nil {
			cleanupFirst()
		}
		return early, nil
	}

	return rewriteBundlesPooled(root, targets, encrypt, first, cleanupFirst, launch, progress, cancel)
}

// Scanner IDs of complete unencrypted IoStore mods whose listings leave
// /Game/Marvel/Characters. Missing identities or path listings are skipped.
func FindModsNeedingEncryption(
	entries []discovery.Entry,
	identities map[string]modtype.Identity,
	paths map[string][]string,
) []string {
	var ids []string
	for _, entry := range entries {
		if validateEncryptableEntry(entry) != nil {
			continue
		}
		if identities[entry.ID].Encrypted {
			continue
		}
		listing, ok := paths[entry.ID]
		if !ok {
			continue
		}
		if modtype.RequiresIoStoreEncryption(listing) {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}

func planEncryption(
	modRoot string,
	entryIDs []string,
	encrypt bool,
	caller ArchiveCaller,
) (string, []discovery.Entry, EncryptionBatchResult, bool, error) {
	if len(entryIDs) == 0 {
		return "", nil, EncryptionBatchResult{}, false, fmt.Errorf("no mods selected")
	}

	library, err := discovery.Scan(modRoot)
	if err != nil {
		return "", nil, EncryptionBatchResult{}, false, fmt.Errorf("scan mod library before encryption: %w", err)
	}

	root, err := filepath.Abs(library.Root)
	if err != nil {
		return "", nil, EncryptionBatchResult{}, false, fmt.Errorf("resolve mod root: %w", err)
	}

	targets := make([]discovery.Entry, 0, len(entryIDs))
	var encryptedCount int
	for _, id := range entryIDs {
		entry, err := findEntry(library.Entries, id)
		if err != nil {
			return "", nil, EncryptionBatchResult{}, false, err
		}
		if err := validateEncryptableEntry(entry); err != nil {
			return "", nil, EncryptionBatchResult{}, false, err
		}

		utocPath := filepath.Join(root, filepath.FromSlash(entry.Sidecars.UTOC))
		encrypted, err := uassettool.IsIoStoreEncrypted(caller, utocPath)
		if err != nil {
			return "", nil, EncryptionBatchResult{}, false, fmt.Errorf("check encryption for %q: %w", entry.DisplayName, err)
		}
		if encrypted {
			encryptedCount++
		}
		targets = append(targets, entry)
	}

	if encryptedCount != 0 && encryptedCount != len(targets) {
		return "", nil, EncryptionBatchResult{}, false, ErrEncryptionMixedState
	}
	alreadyEncrypted := encryptedCount == len(targets)
	if alreadyEncrypted == encrypt {
		return root, nil, EncryptionBatchResult{Succeeded: append([]string(nil), entryIDs...)}, true, nil
	}
	return root, targets, EncryptionBatchResult{}, false, nil
}

func rewriteBundlesPooled(
	root string,
	targets []discovery.Entry,
	encrypt bool,
	first ArchiveCaller,
	cleanupFirst func(),
	launch ArchiveCallerFactory,
	progress func(EncryptionProgress),
	cancel <-chan struct{},
) (EncryptionBatchResult, error) {
	size := uassettool.DefaultWorkerPoolSizeForLibrary(len(targets))
	if size > len(targets) {
		size = len(targets)
	}
	if size < 1 {
		size = 1
	}

	workers := []encryptionWorker{{caller: first, cleanup: cleanupFirst}}
	for i := 1; i < size; i++ {
		caller, cleanup, err := launch()
		if err != nil {
			for _, worker := range workers {
				if worker.cleanup != nil {
					worker.cleanup()
				}
			}
			return EncryptionBatchResult{}, fmt.Errorf("start encryption worker: %w", err)
		}
		workers = append(workers, encryptionWorker{caller: caller, cleanup: cleanup})
	}

	jobs := make(chan discovery.Entry)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var started atomic.Int32
	result := EncryptionBatchResult{}
	sawCancel := false

	for _, worker := range workers {
		wg.Add(1)
		go func(caller ArchiveCaller, cleanup func()) {
			defer wg.Done()
			if cleanup != nil {
				defer cleanup()
			}
			for entry := range jobs {
				if cancelled(cancel) {
					mu.Lock()
					sawCancel = true
					result.Failed = append(result.Failed, EncryptionFailure{
						EntryID: entry.ID,
						Message: ErrEncryptionCancelled.Error(),
					})
					mu.Unlock()
					continue
				}
				current := int(started.Add(1))
				if progress != nil {
					progress(EncryptionProgress{
						Current:     current,
						Total:       len(targets),
						EntryID:     entry.ID,
						DisplayName: entry.DisplayName,
					})
				}
				if err := rewriteBundleEncryption(root, entry, encrypt, caller); err != nil {
					mu.Lock()
					result.Failed = append(result.Failed, EncryptionFailure{EntryID: entry.ID, Message: err.Error()})
					mu.Unlock()
					continue
				}
				mu.Lock()
				result.Succeeded = append(result.Succeeded, entry.ID)
				mu.Unlock()
			}
		}(worker.caller, worker.cleanup)
	}

	sent := 0
	for _, entry := range targets {
		if cancelled(cancel) {
			break
		}
		jobs <- entry
		sent++
	}
	close(jobs)
	wg.Wait()

	if sent < len(targets) {
		for _, entry := range targets[sent:] {
			result.Failed = append(result.Failed, EncryptionFailure{
				EntryID: entry.ID,
				Message: ErrEncryptionCancelled.Error(),
			})
		}
		return result, ErrEncryptionCancelled
	}
	if sawCancel {
		return result, ErrEncryptionCancelled
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

// Both actions change staged copies. The live bundle changes only after verification.
func rewriteBundleEncryption(root string, entry discovery.Entry, encrypt bool, caller ArchiveCaller) error {
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

	// Stage under a plain .pak name so the worker finds the sibling .utoc
	// next to it even when the live primary is disabled (.pak_crateoff).
	stagedPak := filepath.Join(workDir, "bundle.pak")
	stagedUtoc := filepath.Join(workDir, "bundle.utoc")
	stagedUcas := filepath.Join(workDir, "bundle.ucas")
	for live, staged := range map[string]string{
		primaryAbs: stagedPak,
		utocAbs:    stagedUtoc,
		ucasAbs:    stagedUcas,
	} {
		if err := requireRegularFile(live, "live bundle file"); err != nil {
			return err
		}
		if err := copyRegularFile(live, staged); err != nil {
			return fmt.Errorf("stage bundle file for encryption: %w", err)
		}
	}

	if err := RewriteCompanionPak(workDir, stagedPak, stagedUtoc, caller); err != nil {
		return fmt.Errorf("clean staged companion PAK: %w", err)
	}
	if encrypt {
		if _, err := uassettool.EncryptIoStoreDirect(caller, stagedPak, uassettool.MarvelRivalsAESKey); err != nil {
			return fmt.Errorf("encrypt IoStore: %w", err)
		}
	} else if err := uassettool.DecryptIoStoreDirect(caller, stagedUtoc, uassettool.MarvelRivalsAESKey); err != nil {
		return fmt.Errorf("decrypt IoStore: %w", err)
	}
	encrypted, err := uassettool.IsIoStoreEncrypted(caller, stagedUtoc)
	if err != nil {
		return fmt.Errorf("verify encryption: %w", err)
	}
	if encrypted != encrypt {
		return fmt.Errorf("verify encryption: container did not reach the requested state")
	}

	rebuilt := map[string]string{
		primaryAbs: stagedPak,
		utocAbs:    stagedUtoc,
		ucasAbs:    stagedUcas,
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
