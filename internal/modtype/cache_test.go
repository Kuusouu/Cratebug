package modtype

import (
	"sync"
	"testing"
	"time"
)

func TestCachePutAndGet(t *testing.T) {
	// Arrange
	cache := NewCache()
	mtime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	want := CachedClassification{
		Identity: Identity{
			Category:      CategoryMesh,
			CharacterName: "Luna Snow",
			SkinName:      "Default",
		},
		Paths: []string{"Marvel/Content/Marvel/Characters/1044/SK_Luna.uasset"},
	}

	// Act
	cache.Put("mod-1", mtime, want)
	got, ok := cache.Get("mod-1", mtime)

	// Assert
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got.Identity != want.Identity {
		t.Errorf("Get().Identity = %#v, want %#v", got.Identity, want.Identity)
	}
	if len(got.Paths) != 1 || got.Paths[0] != want.Paths[0] {
		t.Errorf("Get().Paths = %v, want %v", got.Paths, want.Paths)
	}
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cache.Len())
	}
}

func TestCacheMissOnEntryID(t *testing.T) {
	// Arrange
	cache := NewCache()
	mtime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	cache.Put("mod-1", mtime, CachedClassification{Identity: Identity{Category: CategoryMesh}})

	// Act
	_, ok := cache.Get("mod-2", mtime)

	// Assert
	if ok {
		t.Error("Get() returned true for unknown entry, want false")
	}
}

func TestCacheMissOnMTimeChange(t *testing.T) {
	// Arrange
	cache := NewCache()
	oldTime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	cache.Put("mod-1", oldTime, CachedClassification{Identity: Identity{Category: CategoryMesh}})

	// Act
	_, ok := cache.Get("mod-1", newTime)

	// Assert
	if ok {
		t.Error("Get() returned true for modified mtime, want false")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	// Arrange
	cache := NewCache()
	mtime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup

	// Act (concurrent puts and gets)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			cache.Put("mod-1", mtime, CachedClassification{Identity: Identity{Category: CategoryTexture}})
		}(i)
		go func(id int) {
			defer wg.Done()
			_, _ = cache.Get("mod-1", mtime)
		}(i)
	}
	wg.Wait()

	// Assert
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cache.Len())
	}
}
