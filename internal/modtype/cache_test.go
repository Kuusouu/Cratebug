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
	want := Identity{
		Category:      CategoryMesh,
		CharacterName: "Luna Snow",
		SkinName:      "Default",
	}

	// Act
	cache.Put("mod-1", mtime, want)
	got, ok := cache.Get("mod-1", mtime)

	// Assert
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got != want {
		t.Errorf("Get() = %#v, want %#v", got, want)
	}
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cache.Len())
	}
}

func TestCacheMissOnEntryID(t *testing.T) {
	// Arrange
	cache := NewCache()
	mtime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	cache.Put("mod-1", mtime, Identity{Category: CategoryMesh})

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
	cache.Put("mod-1", oldTime, Identity{Category: CategoryMesh})

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
			cache.Put("mod-1", mtime, Identity{Category: CategoryTexture})
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
