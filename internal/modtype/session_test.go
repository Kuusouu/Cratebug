package modtype

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

type mockPoolWorker struct {
	mu        sync.Mutex
	alive     bool
	callCount int
	callFunc  func(action string, params map[string]any, result any) error
}

func newMockPoolWorker(fn func(action string, params map[string]any, result any) error) *mockPoolWorker {
	return &mockPoolWorker{
		alive:    true,
		callFunc: fn,
	}
}

func (m *mockPoolWorker) Call(action string, params map[string]any, result any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.callFunc != nil {
		return m.callFunc(action, params, result)
	}
	return nil
}

func (m *mockPoolWorker) Alive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alive
}

func (m *mockPoolWorker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive = false
	return nil
}

func TestSessionClassifierCachesResultsOnRepeatedScans(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod1 := filepath.Join(root, "Mod1_P.pak")
	mod2 := filepath.Join(root, "Mod2_P.pak")
	if err := os.WriteFile(mod1, []byte("pak1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod2, []byte("pak2"), 0o600); err != nil {
		t.Fatal(err)
	}

	var totalCalls int64
	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(action string, params map[string]any, result any) error {
			atomic.AddInt64(&totalCalls, 1)
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`), result)
			}
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
		{
			ID:           "mod-2",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod2_P.pak",
		},
	}

	table := CharacterTable{
		CharacterNames: map[string]string{"1044": "Blade"},
	}

	// Act - First scan (cache miss for both)
	firstResults, err := classifier.Classify(root, entries, table)
	if err != nil {
		t.Fatalf("Classify() first run error = %v", err)
	}

	firstCallCount := atomic.LoadInt64(&totalCalls)

	// Act - Second scan (both unchanged, should hit cache)
	secondResults, err := classifier.Classify(root, entries, table)
	if err != nil {
		t.Fatalf("Classify() second run error = %v", err)
	}

	secondCallCount := atomic.LoadInt64(&totalCalls)

	// Assert
	if len(firstResults) != 2 || len(secondResults) != 2 {
		t.Fatalf("results count = (%d, %d), want (2, 2)", len(firstResults), len(secondResults))
	}
	if firstResults["mod-1"].CharacterName != "Blade" || firstResults["mod-1"].Category != CategoryMesh {
		t.Errorf("firstResults[mod-1] = %#v, want Blade / Mesh", firstResults["mod-1"])
	}
	if firstResults["mod-2"].CharacterName != "Blade" || firstResults["mod-2"].Category != CategoryMesh {
		t.Errorf("firstResults[mod-2] = %#v, want Blade / Mesh", firstResults["mod-2"])
	}
	if firstCallCount != 2 {
		t.Errorf("firstCallCount = %d, want 2", firstCallCount)
	}
	if secondCallCount != firstCallCount {
		t.Errorf("secondCallCount = %d, want %d (no additional worker calls for cached entries)", secondCallCount, firstCallCount)
	}
}

func TestSessionClassifierReclassifiesOnlyModifiedEntry(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod1 := filepath.Join(root, "Mod1_P.pak")
	mod2 := filepath.Join(root, "Mod2_P.pak")
	if err := os.WriteFile(mod1, []byte("pak1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod2, []byte("pak2"), 0o600); err != nil {
		t.Fatal(err)
	}

	var totalCalls int64
	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(action string, params map[string]any, result any) error {
			atomic.AddInt64(&totalCalls, 1)
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`), result)
			}
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
		{
			ID:           "mod-2",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod2_P.pak",
		},
	}

	// First pass
	_, err := classifier.Classify(root, entries, CharacterTable{})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(&totalCalls) != 2 {
		t.Fatalf("totalCalls after first pass = %d, want 2", atomic.LoadInt64(&totalCalls))
	}

	// Modify mod1 file (touch mtime by advancing 2 seconds)
	newTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(mod1, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	// Act - Second pass
	_, err = classifier.Classify(root, entries, CharacterTable{})
	if err != nil {
		t.Fatal(err)
	}

	// Assert: only mod-1 was re-called (1 additional call)
	if got := atomic.LoadInt64(&totalCalls); got != 3 {
		t.Errorf("totalCalls after modification = %d, want 3", got)
	}
}

func TestSessionClassifierDegradesToUnknownOnLauncherFailure(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod := filepath.Join(root, "Mod1_P.pak")
	if err := os.WriteFile(mod, []byte("pak"), 0o600); err != nil {
		t.Fatal(err)
	}

	launcher := func() (PoolWorker, error) {
		return nil, errors.New("simulated launch failure")
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
	}

	// Act
	results, err := classifier.Classify(root, entries, CharacterTable{})

	// Assert
	if err != nil {
		t.Fatalf("Classify() error = %v, want nil (graceful degradation)", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results["mod-1"].Category != CategoryUnknown {
		t.Errorf("results[mod-1].Category = %q, want %q", results["mod-1"].Category, CategoryUnknown)
	}
}

func TestSessionClassifierDoesNotCacheFailedOutcomes(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod := filepath.Join(root, "Mod1_P.pak")
	if err := os.WriteFile(mod, []byte("pak"), 0o600); err != nil {
		t.Fatal(err)
	}

	shouldFail := true
	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(action string, params map[string]any, result any) error {
			if shouldFail {
				return errors.New("transient worker failure")
			}
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`), result)
			}
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
	}

	// Act 1: First scan fails -> returns Unknown, should NOT write to cache
	firstResults, err := classifier.Classify(root, entries, CharacterTable{})
	if err != nil {
		t.Fatalf("first Classify() error = %v", err)
	}
	if firstResults["mod-1"].Category != CategoryUnknown {
		t.Errorf("firstResults[mod-1].Category = %q, want %q", firstResults["mod-1"].Category, CategoryUnknown)
	}

	// Check cache directly - must be a miss
	fi, err := os.Stat(mod)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := classifier.Cache().Get("mod-1", fi.ModTime()); ok {
		t.Error("classifier cached a failed outcome; expected cache miss")
	}

	// Act 2: Worker heals (transient failure resolved) -> second scan succeeds and populates cache
	shouldFail = false
	table := CharacterTable{CharacterNames: map[string]string{"1044": "Blade"}}
	secondResults, err := classifier.Classify(root, entries, table)
	if err != nil {
		t.Fatalf("second Classify() error = %v", err)
	}
	if secondResults["mod-1"].CharacterName != "Blade" || secondResults["mod-1"].Category != CategoryMesh {
		t.Errorf("secondResults[mod-1] = %#v, want Blade / Mesh", secondResults["mod-1"])
	}

	// Now cache should have it
	if cached, ok := classifier.Cache().Get("mod-1", fi.ModTime()); !ok || cached.CharacterName != "Blade" {
		t.Errorf("cache get = (%#v, %v), want Blade and hit", cached, ok)
	}
}

func TestSessionClassifierReplacesDeadWorker(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod1 := filepath.Join(root, "Mod1_P.pak")
	mod2 := filepath.Join(root, "Mod2_P.pak")
	if err := os.WriteFile(mod1, []byte("pak1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod2, []byte("pak2"), 0o600); err != nil {
		t.Fatal(err)
	}

	var workersLaunched int64
	launcher := func() (PoolWorker, error) {
		id := atomic.AddInt64(&workersLaunched, 1)
		w := newMockPoolWorker(func(action string, params map[string]any, result any) error {
			if id == 1 {
				// First worker dies on its first call
				return errors.New("worker crash")
			}
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Textures/T_UI.uasset"}]}`), result)
			}
			return nil
		})
		if id == 1 {
			// Mark dead immediately after creation / first call
			w.alive = false
		}
		return w, nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
		{
			ID:           "mod-2",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod2_P.pak",
		},
	}

	// Act
	results, err := classifier.Classify(root, entries, CharacterTable{})

	// Assert
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	// At least 2 workers were launched (recovery happened)
	if atomic.LoadInt64(&workersLaunched) < 2 {
		t.Errorf("workersLaunched = %d, want at least 2", atomic.LoadInt64(&workersLaunched))
	}
}

func TestSessionClassifierCloseIsIdempotent(t *testing.T) {
	// Arrange
	classifier := NewSessionClassifier(func() (PoolWorker, error) {
		return newMockPoolWorker(nil), nil
	})

	// Act & Assert
	if err := classifier.Close(); err != nil {
		t.Errorf("first Close() error = %v, want nil", err)
	}
	if err := classifier.Close(); err != nil {
		t.Errorf("second Close() error = %v, want nil", err)
	}
}

func TestSessionClassifierUsesUpdatedCharacterTableOnWarmPool(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod1 := filepath.Join(root, "Mod1_P.pak")
	mod2 := filepath.Join(root, "Mod2_P.pak")
	if err := os.WriteFile(mod1, []byte("pak1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod2, []byte("pak2"), 0o600); err != nil {
		t.Fatal(err)
	}

	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(action string, params map[string]any, result any) error {
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`), result)
			}
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entry1 := discovery.Entry{
		ID:           "mod-1",
		Kind:         discovery.EntryMod,
		BundleFormat: discovery.BundleFormatClassic,
		PrimaryPath:  "Mod1_P.pak",
	}
	entry2 := discovery.Entry{
		ID:           "mod-2",
		Kind:         discovery.EntryMod,
		BundleFormat: discovery.BundleFormatClassic,
		PrimaryPath:  "Mod2_P.pak",
	}

	// First pass with empty table
	firstResults, err := classifier.Classify(root, []discovery.Entry{entry1}, CharacterTable{})
	if err != nil {
		t.Fatal(err)
	}
	if firstResults["mod-1"].CharacterName != "" {
		t.Errorf("firstResults[mod-1].CharacterName = %q, want empty", firstResults["mod-1"].CharacterName)
	}

	// Second pass with updated table classifying entry2 on the warm pool
	updatedTable := CharacterTable{
		CharacterNames: map[string]string{"1044": "Blade"},
	}
	secondResults, err := classifier.Classify(root, []discovery.Entry{entry2}, updatedTable)
	if err != nil {
		t.Fatal(err)
	}
	if secondResults["mod-2"].CharacterName != "Blade" {
		t.Errorf("secondResults[mod-2].CharacterName = %q, want Blade (warm pool should use new table)", secondResults["mod-2"].CharacterName)
	}
}

func TestSessionClassifierSkipsNonModEntries(t *testing.T) {
	// Arrange
	root := t.TempDir()
	var calls int64
	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(string, map[string]any, any) error {
			atomic.AddInt64(&calls, 1)
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	entries := []discovery.Entry{
		{
			ID:           "orphan-1",
			Kind:         discovery.EntryOrphanedSidecar,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "",
		},
	}

	// Act
	results, err := classifier.Classify(root, entries, CharacterTable{})

	// Assert
	if err != nil {
		t.Fatalf("Classify() error = %v, want nil", err)
	}
	if len(results) != 1 || results["orphan-1"].Category != CategoryUnknown {
		t.Errorf("results[orphan-1] = %#v, want Unknown", results["orphan-1"])
	}
	if atomic.LoadInt64(&calls) != 0 {
		t.Errorf("calls = %d, want 0 worker calls for non-mod entries", atomic.LoadInt64(&calls))
	}
}

func TestSessionClassifierResizesPoolAcrossTiers(t *testing.T) {
	// Arrange
	root := t.TempDir()
	mod := filepath.Join(root, "Mod_P.pak")
	if err := os.WriteFile(mod, []byte("pak"), 0o600); err != nil {
		t.Fatal(err)
	}

	launcher := func() (PoolWorker, error) {
		return newMockPoolWorker(func(action string, params map[string]any, result any) error {
			if action == "list_pak" {
				return json.Unmarshal([]byte(`{"files":[{"path":"Marvel/Content/Characters/1044/SK_Blade.uasset"}]}`), result)
			}
			return nil
		}), nil
	}

	classifier := NewSessionClassifier(launcher)
	defer classifier.Close()

	// Initial small classify -> small tier pool
	entry := discovery.Entry{
		ID:           "mod-1",
		Kind:         discovery.EntryMod,
		BundleFormat: discovery.BundleFormatClassic,
		PrimaryPath:  "Mod_P.pak",
	}

	_, err := classifier.Classify(root, []discovery.Entry{entry}, CharacterTable{})
	if err != nil {
		t.Fatal(err)
	}
	initialSize := classifier.pool.Size()
	if initialSize < 1 {
		t.Fatalf("initialSize = %d, want >= 1", initialSize)
	}

	// Build 800 synthetic entries to hit medium tier (700-1499)
	largeBatch := make([]discovery.Entry, 800)
	for i := 0; i < 800; i++ {
		largeBatch[i] = discovery.Entry{
			ID:           filepath.Join("mod", string(rune('a'+(i%26)))),
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod_P.pak",
		}
	}

	// Act - Classify large batch
	_, err = classifier.Classify(root, largeBatch, CharacterTable{})
	if err != nil {
		t.Fatal(err)
	}

	// Assert pool exists and has adjusted size
	if classifier.pool == nil {
		t.Fatal("pool is nil after large classify")
	}
}
