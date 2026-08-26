package modtype

import (
	"sync"
	"time"
)

type cacheKey struct {
	entryID   string
	modTimeNs int64
}

// Bundles a mod's derived Identity with the internal asset paths that
// produced it. Paths is retained so a later asset conflict scan (Phase 9)
// can reuse the same UAssetToolRivals listing this classification pass
// already fetched, instead of issuing a second listing call per mod.
type CachedClassification struct {
	Identity Identity
	Paths    []string
}

// Cache provides thread-safe in-memory caching of mod classification results,
// keyed by scanner entry ID and the modification time of the primary file.
type Cache struct {
	mu      sync.RWMutex
	entries map[cacheKey]CachedClassification
}

// NewCache creates an empty in-memory classification cache.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[cacheKey]CachedClassification),
	}
}

// Get looks up a cached classification result for the given entry ID and modification time.
func (c *Cache) Get(entryID string, modTime time.Time) (CachedClassification, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.entries[cacheKey{
		entryID:   entryID,
		modTimeNs: modTime.UnixNano(),
	}]
	return result, ok
}

// Put records a classification result in the cache.
func (c *Cache) Put(entryID string, modTime time.Time, result CachedClassification) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[cacheKey{
		entryID:   entryID,
		modTimeNs: modTime.UnixNano(),
	}] = result
}

// Len returns the number of items stored in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}
