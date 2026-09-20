package watcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherSetRootAndClose(t *testing.T) {
	dir := t.TempDir()

	w, err := New(Options{
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}
	if w.Root() != dir {
		t.Fatalf("Root() = %q, want %q", w.Root(), dir)
	}

	if err := w.SetRoot(""); err != nil {
		t.Fatalf("SetRoot(\"\") error = %v", err)
	}
	if w.Root() != "" {
		t.Fatalf("Root() = %q, want empty", w.Root())
	}
}

func TestWatcherDebouncesFileChanges(t *testing.T) {
	dir := t.TempDir()

	var triggerCount atomic.Int32
	done := make(chan struct{}, 5)

	w, err := New(Options{
		Debounce: 80 * time.Millisecond,
		OnChange: func() {
			triggerCount.Add(1)
			select {
			case done <- struct{}{}:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}

	// Write rapid burst of files
	pak1 := filepath.Join(dir, "ModA_P.pak")
	utoc1 := filepath.Join(dir, "ModA_P.utoc")
	ucas1 := filepath.Join(dir, "ModA_P.ucas")

	_ = os.WriteFile(pak1, []byte("pak1"), 0o600)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(utoc1, []byte("utoc1"), 0o600)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(ucas1, []byte("ucas1"), 0o600)

	// Wait for debounce timer
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for onChange notification")
	}

	// Give a small grace period to ensure only 1 debounced trigger fired
	time.Sleep(120 * time.Millisecond)
	if count := triggerCount.Load(); count != 1 {
		t.Fatalf("triggerCount = %d, want 1 (debounced)", count)
	}
}

func TestWatcherTracksDynamicSubdirectories(t *testing.T) {
	dir := t.TempDir()

	done := make(chan struct{}, 5)

	w, err := New(Options{
		Debounce: 80 * time.Millisecond,
		OnChange: func() {
			select {
			case done <- struct{}{}:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}

	// Create new subfolder
	subDir := filepath.Join(dir, "SubFolder")
	if err := os.Mkdir(subDir, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	// Wait for directory addition to settle
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	// Write a mod file inside the new subfolder
	modFile := filepath.Join(subDir, "SubMod_P.pak")
	_ = os.WriteFile(modFile, []byte("submod"), 0o600)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for subfolder mod notification")
	}
}

func TestWatcherIgnoresNoiseFiles(t *testing.T) {
	dir := t.TempDir()

	var triggerCount atomic.Int32
	w, err := New(Options{
		Debounce: 50 * time.Millisecond,
		OnChange: func() {
			triggerCount.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}

	// Write noise files
	_ = os.WriteFile(filepath.Join(dir, "temp.tmp"), []byte("noise"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("{}"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "nexus.key"), []byte("secret"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("read me"), 0o600)

	time.Sleep(150 * time.Millisecond)
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d, want 0 for noise files", count)
	}
}

func TestWatcherSuppress(t *testing.T) {
	dir := t.TempDir()

	var triggerCount atomic.Int32
	w, err := New(Options{
		Debounce: 50 * time.Millisecond,
		OnChange: func() {
			triggerCount.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	if err := w.SetRoot(dir); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}

	w.Suppress(func() {
		_ = os.WriteFile(filepath.Join(dir, "InternalMod_P.pak"), []byte("internal"), 0o600)
	})

	time.Sleep(300 * time.Millisecond)
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d, want 0 during suppression", count)
	}
}
