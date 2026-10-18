package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/watcher"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Returned from a metadataGate.update callback when the document did not
// change, so update skips the save.
var errMetadataUnchanged = errors.New("metadata unchanged")

// Holds the backend pieces that more than one feature depends on. It has no
// exported methods, so Wails never binds anything on it.
type core struct {
	runtime  *runtimeHost
	watcher  *watcher.Watcher
	game     mutation.GameRunningChecker
	meta     *metadataGate
	catalog  *catalog
	detector *gamedetect.Registry
}

// Holds the Wails runtime context, which only exists after startup.
type runtimeHost struct {
	mu  sync.Mutex
	ctx context.Context
}

// Serializes every metadata read and write. Wails runs each binding call on
// its own goroutine, so an unguarded load, modify, and save from two calls at
// once would lose one of the writes.
type metadataGate struct {
	mu    sync.Mutex
	store metadata.Store
}

// Classifies mods against a character table loaded on first use.
type catalog struct {
	classifier *modtype.SessionClassifier
	mu         sync.Mutex
	table      modtype.CharacterTable
	loaded     bool
}

func (h *runtimeHost) set(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ctx = ctx
}

// Returns the Wails runtime context, or nil before startup. Dialogs and
// window calls need the real context, so they use this and fail without it.
func (h *runtimeHost) current() context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ctx
}

// Returns the Wails runtime context for cancellable work, falling back to a
// background context before startup and in tests.
func (h *runtimeHost) context() context.Context {
	if ctx := h.current(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// Sends a frontend event. Events before startup are dropped because there is
// no frontend to receive them yet.
func (h *runtimeHost) emit(event string, data ...any) {
	if ctx := h.current(); ctx != nil {
		wailsRuntime.EventsEmit(ctx, event, data...)
	}
}

func newMetadataGate(store metadata.Store) *metadataGate {
	return &metadataGate{store: store}
}

// Returns the persisted document and whether it had to be recovered.
func (g *metadataGate) load() (metadata.Document, metadata.Recovery) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.store.Load()
}

// Returns the persisted document, discarding the recovery details that
// LoadMetadata reports to the frontend.
func (g *metadataGate) read() metadata.Document {
	doc, _ := g.load()
	return doc
}

// Loads the document, applies change, and saves the result, all under one
// lock. A change that returns an error is not saved; errMetadataUnchanged
// skips the save without reporting an error.
func (g *metadataGate) update(change func(*metadata.Document) error) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	doc, _ := g.store.Load()
	if err := change(&doc); err != nil {
		if errors.Is(err, errMetadataUnchanged) {
			return nil
		}
		return err
	}
	return g.store.Save(doc)
}

// A nil table loads the cached or bundled table on first use. Tests pass one
// to stay offline.
func newCatalog(classifier *modtype.SessionClassifier, table *modtype.CharacterTable) *catalog {
	c := &catalog{classifier: classifier}
	if table != nil {
		c.table = *table
		c.loaded = true
	}
	return c
}

func (c *catalog) characterTable() modtype.CharacterTable {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loaded {
		return c.table
	}

	cachePath, _ := modtype.DefaultCharacterTableCachePath()
	c.table = modtype.LoadCharacterTable(context.Background(), cachePath, modtype.DefaultCharacterTableMaxAge)
	c.loaded = true
	return c.table
}

func (c *catalog) classify(modRoot string, entries []discovery.Entry) (map[string]modtype.Identity, error) {
	return c.classifier.Classify(modRoot, entries, c.characterTable())
}

// Returns the internal asset paths cached for entry by its last classify.
func (c *catalog) pathsFor(modRoot string, entry discovery.Entry) ([]string, bool) {
	return c.classifier.PathsForEntry(modRoot, entry)
}

func (c *catalog) close() error {
	return c.classifier.Close()
}

// Blocks a mutation while Marvel Rivals runs. A nil checker allows it.
func (c *core) requireGameNotRunning() error {
	if c.game == nil {
		return nil
	}
	running, err := c.game.IsGameRunning()
	if err != nil {
		return fmt.Errorf("check whether Marvel Rivals is running: %w", err)
	}
	if running {
		return mutation.ErrGameRunning
	}
	return nil
}

// Pauses the library watcher while fn changes files, so Cratebug's own
// writes do not come back as external changes.
func suppressWatcherResult[T any](c *core, fn func() (T, error)) (T, error) {
	if c != nil && c.watcher != nil {
		c.watcher.Pause()
		defer c.watcher.Resume()
	}
	return fn()
}
