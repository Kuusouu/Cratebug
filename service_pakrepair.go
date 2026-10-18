package main

import (
	"fmt"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Encryption and companion PAK cleanup, bound to the frontend.
type PakRepairService struct {
	*core

	// Guard in-flight cancellation so the CancelEncryption and
	// CancelCompanionCleanup bindings can reach the running operation.
	encryptMu       sync.Mutex
	encryptCancel   chan struct{}
	companionMu     sync.Mutex
	companionCancel chan struct{}
	companionCache  *mutation.CompanionCache
}

// Starts the uasset worker on first use, so a scan that finds nothing to
// inspect never launches one.
type lazyWorkerCaller struct {
	mu     sync.Mutex
	worker *uassettool.Worker
}

func newPakRepairService(c *core) *PakRepairService {
	return &PakRepairService{core: c, companionCache: mutation.NewCompanionCache()}
}

func (l *lazyWorkerCaller) Call(action string, params map[string]any, result any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.worker == nil {
		w, err := uassettool.NewWriteWorker(nil)
		if err != nil {
			return err
		}
		l.worker = w
	}
	return l.worker.Call(action, params, result)
}

func (l *lazyWorkerCaller) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.worker != nil {
		_ = l.worker.Close()
		l.worker = nil
	}
}

// Encrypts or decrypts each complete IoStore bundle without a legacy asset conversion.
// The Marvel Rivals AES key stays in Go. The frontend receives only the
// encrypted flag after the next classify. Emits encrypt:progress events.
func (s *PakRepairService) SetModEncryption(modRoot string, entryIDs []string, encrypt bool) (mutation.EncryptionBatchResult, error) {
	return suppressWatcherResult(s.core, func() (mutation.EncryptionBatchResult, error) {
		if err := s.requireGameNotRunning(); err != nil {
			return mutation.EncryptionBatchResult{}, err
		}

		s.encryptMu.Lock()
		s.encryptCancel = make(chan struct{})
		cancel := s.encryptCancel
		s.encryptMu.Unlock()
		defer func() {
			s.encryptMu.Lock()
			s.encryptCancel = nil
			s.encryptMu.Unlock()
		}()

		launch := func() (mutation.ArchiveCaller, func(), error) {
			worker, err := uassettool.NewWriteWorker(nil)
			if err != nil {
				return nil, nil, err
			}
			return worker, func() { _ = worker.Close() }, nil
		}

		return mutation.SetModEncryptionPooled(modRoot, entryIDs, encrypt, launch, func(progress mutation.EncryptionProgress) {
			s.runtime.emit("encrypt:progress", progress)
		}, cancel)
	})
}

// Stops an in-progress SetModEncryption after in-flight pool jobs finish.
func (s *PakRepairService) CancelEncryption() {
	s.encryptMu.Lock()
	defer s.encryptMu.Unlock()
	if s.encryptCancel != nil {
		close(s.encryptCancel)
		s.encryptCancel = nil
	}
}

// Lists complete unencrypted IoStore IDs whose classify listing leaves Characters.
func (s *PakRepairService) FindModsNeedingEncryption(modRoot string) ([]string, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return nil, fmt.Errorf("scan mod library before required-encryption check: %w", err)
	}
	identities, err := s.catalog.classify(modRoot, library.Entries)
	if err != nil {
		return nil, fmt.Errorf("classify library before required-encryption check: %w", err)
	}
	paths := make(map[string][]string, len(library.Entries))
	for _, entry := range library.Entries {
		if listing, ok := s.catalog.pathsFor(modRoot, entry); ok {
			paths[entry.ID] = listing
		}
	}
	return mutation.FindModsNeedingEncryption(library.Entries, identities, paths), nil
}

// Lists scanner IDs whose companion PAK still contains chunknames or patched_files.
func (s *PakRepairService) FindUnsupportedCompanionPaks(modRoot string) ([]string, error) {
	lazy := &lazyWorkerCaller{}
	defer lazy.Close()
	return mutation.FindUnsupportedCompanionPaksWithCache(modRoot, lazy, s.companionCache)
}

// Rewrites each dirty companion PAK one at a time. Emits companion:progress.
func (s *PakRepairService) StripCompanionPaks(modRoot string, entryIDs []string) (mutation.CompanionCleanupResult, error) {
	return suppressWatcherResult(s.core, func() (mutation.CompanionCleanupResult, error) {
		if err := s.requireGameNotRunning(); err != nil {
			return mutation.CompanionCleanupResult{}, err
		}

		worker, err := uassettool.NewWriteWorker(nil)
		if err != nil {
			return mutation.CompanionCleanupResult{}, err
		}
		defer func() {
			_ = worker.Close() // Worker.Close waits for shutdown and always returns nil.
		}()

		s.companionMu.Lock()
		s.companionCancel = make(chan struct{})
		cancel := s.companionCancel
		s.companionMu.Unlock()
		defer func() {
			s.companionMu.Lock()
			s.companionCancel = nil
			s.companionMu.Unlock()
		}()

		return mutation.StripCompanionPaks(modRoot, entryIDs, worker, func(progress mutation.CompanionCleanupProgress) {
			s.runtime.emit("companion:progress", progress)
		}, cancel)
	})
}

// Stops an in-progress StripCompanionPaks after the current bundle finishes.
func (s *PakRepairService) CancelCompanionCleanup() {
	s.companionMu.Lock()
	defer s.companionMu.Unlock()
	if s.companionCancel != nil {
		close(s.companionCancel)
		s.companionCancel = nil
	}
}
