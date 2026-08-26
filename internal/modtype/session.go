package modtype

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// SessionClassifier coordinates mod classification across a session, maintaining
// an in-memory mtime cache and a warm, dynamically-sized worker pool.
type SessionClassifier struct {
	mu       sync.Mutex
	cache    *Cache
	pool     *WorkerPool
	launcher WorkerLauncher
}

// NewSessionClassifier creates a SessionClassifier with an empty cache and the given worker launcher.
func NewSessionClassifier(launcher WorkerLauncher) *SessionClassifier {
	return NewSessionClassifierWithCache(NewCache(), launcher)
}

// NewSessionClassifierWithCache creates a SessionClassifier with the specified cache.
func NewSessionClassifierWithCache(cache *Cache, launcher WorkerLauncher) *SessionClassifier {
	if cache == nil {
		cache = NewCache()
	}
	return &SessionClassifier{
		cache:    cache,
		launcher: launcher,
	}
}

// Classify classifies all provided entries relative to root, serving unchanged entries
// from cache and processing misses concurrently through the session worker pool.
func (s *SessionClassifier) Classify(root string, entries []discovery.Entry, table CharacterTable) (map[string]Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make(map[string]Identity, len(entries))
	var missJobs []classifyJob

	for _, entry := range entries {
		if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
			results[entry.ID] = Identity{Category: CategoryUnknown}
			continue
		}

		mtime, ok := primaryMTime(root, entry)
		if !ok {
			// If stat fails (e.g. file missing), dispatch as uncacheable miss so it degrades to Unknown
			missJobs = append(missJobs, classifyJob{
				root:  root,
				entry: entry,
				table: table,
			})
			continue
		}

		if cached, ok := s.cache.Get(entry.ID, mtime); ok {
			results[entry.ID] = cached.Identity
			continue
		}

		missJobs = append(missJobs, classifyJob{
			root:  root,
			entry: entry,
			mtime: mtime,
			table: table,
		})
	}

	if len(missJobs) == 0 {
		return results, nil
	}

	targetSize := uassettool.WorkerPoolSizeForLibrary(runtime.NumCPU(), len(missJobs))
	if s.pool == nil || s.pool.Size() != targetSize {
		if s.pool != nil {
			_ = s.pool.Close()
		}
		s.pool = NewWorkerPool(targetSize, s.launcher)
	}

	outcomeChan := make(chan classifyOutcome, len(missJobs))
	for _, job := range missJobs {
		job.result = outcomeChan
		s.pool.Submit(job)
	}

	for i := 0; i < len(missJobs); i++ {
		outcome := <-outcomeChan
		results[outcome.entryID] = outcome.identity
		if !outcome.mtime.IsZero() && !outcome.failed {
			s.cache.Put(outcome.entryID, outcome.mtime, CachedClassification{Identity: outcome.identity, Paths: outcome.paths})
		}
	}

	return results, nil
}

// Cache returns the session classifier's in-memory cache.
func (s *SessionClassifier) Cache() *Cache {
	return s.cache
}

// Paths returns the internal asset path listing retained for entryID the
// last time it was classified at modTime, if any. ok is false when the
// entry has never been classified this session, or was last classified at a
// different modTime — callers should fall back to ListInternalPaths in that
// case rather than treating a miss as an empty listing.
func (s *SessionClassifier) Paths(entryID string, modTime time.Time) ([]string, bool) {
	cached, ok := s.cache.Get(entryID, modTime)
	if !ok {
		return nil, false
	}
	return cached.Paths, true
}

// PathsForEntry resolves entry's primary file mtime against root and looks
// up its retained path listing, the same way Classify itself resolves a
// cache hit. ok is false for a non-mod entry, a missing primary file, or an
// entry not yet classified since its primary file's current mtime — callers
// should treat that as "not yet available", not "no content".
func (s *SessionClassifier) PathsForEntry(root string, entry discovery.Entry) ([]string, bool) {
	if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
		return nil, false
	}
	mtime, ok := primaryMTime(root, entry)
	if !ok {
		return nil, false
	}
	return s.Paths(entry.ID, mtime)
}

// Resolves entry's primary file's current modification time against root.
func primaryMTime(root string, entry discovery.Entry) (time.Time, bool) {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry.PrimaryPath)))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// Close gracefully closes the session worker pool, terminating all active child processes.
func (s *SessionClassifier) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pool != nil {
		err := s.pool.Close()
		s.pool = nil
		return err
	}
	return nil
}
