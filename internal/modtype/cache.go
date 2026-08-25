package modtype

import (
	"sync"
	"time"
)

type cacheKey struct {
	entryID   string
	modTimeNs int64
}

// Cache provides thread-safe in-memory caching of mod classification results,
// keyed by scanner entry ID and the modification time of the primary file.
type Cache struct {
	mu      sync.RWMutex
	entries map[cacheKey]Identity
}

// NewCache creates an empty in-memory classification cache.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[cacheKey]Identity),
	}
}

// Get looks up a cached Identity for the given entry ID and modification time.
func (c *Cache) Get(entryID string, modTime time.Time) (Identity, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	identity, ok := c.entries[cacheKey{
		entryID:   entryID,
		modTimeNs: modTime.UnixNano(),
	}]
	return identity, ok
}

// Put records a classification Identity in the cache.
func (c *Cache) Put(entryID string, modTime time.Time, identity Identity) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[cacheKey{
		entryID:   entryID,
		modTimeNs: modTime.UnixNano(),
	}] = identity
}

// Len returns the number of items stored in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}
