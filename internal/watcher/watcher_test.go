package watcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatcherSetRootAndClose(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	w, err := New(Options{
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	// Act
	setErr := w.SetRoot(dir)
	gotRoot := w.Root()

	// Assert
	if setErr != nil {
		t.Fatalf("SetRoot() error = %v", setErr)
	}
	if gotRoot != dir {
		t.Fatalf("Root() = %q, want %q", gotRoot, dir)
	}

	// Act - clear root
	clearErr := w.SetRoot("")
	emptyRoot := w.Root()

	// Assert
	if clearErr != nil {
		t.Fatalf("SetRoot(\"\") error = %v", clearErr)
	}
	if emptyRoot != "" {
		t.Fatalf("Root() = %q, want empty", emptyRoot)
	}
}

func TestWatcherDebouncesFileChanges(t *testing.T) {
	// Arrange
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

	pak1 := filepath.Join(dir, "ModA_P.pak")
	utoc1 := filepath.Join(dir, "ModA_P.utoc")
	ucas1 := filepath.Join(dir, "ModA_P.ucas")

	// Act - write rapid burst of files
	_ = os.WriteFile(pak1, []byte("pak1"), 0o600)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(utoc1, []byte("utoc1"), 0o600)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(ucas1, []byte("ucas1"), 0o600)

	// Assert
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for onChange notification")
	}

	time.Sleep(120 * time.Millisecond)
	if count := triggerCount.Load(); count != 1 {
		t.Fatalf("triggerCount = %d, want 1 (debounced)", count)
	}
}

func TestWatcherTracksDynamicSubdirectories(t *testing.T) {
	// Arrange
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

	subDir := filepath.Join(dir, "SubFolder")
	if err := os.Mkdir(subDir, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	// Act - write mod file inside dynamically added subdirectory
	modFile := filepath.Join(subDir, "SubMod_P.pak")
	_ = os.WriteFile(modFile, []byte("submod"), 0o600)

	// Assert
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for subfolder mod notification")
	}
}

func TestWatcherIgnoresNoiseFiles(t *testing.T) {
	// Arrange
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

	// Act - write noise files
	_ = os.WriteFile(filepath.Join(dir, "temp.tmp"), []byte("noise"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("{}"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "nexus.key"), []byte("secret"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("read me"), 0o600)

	time.Sleep(150 * time.Millisecond)

	// Assert
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d, want 0 for noise files", count)
	}
}

func TestWatcherSuppress(t *testing.T) {
	// Arrange
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

	// Act - write file during suppression
	w.Suppress(func() {
		_ = os.WriteFile(filepath.Join(dir, "InternalMod_P.pak"), []byte("internal"), 0o600)
	})

	time.Sleep(300 * time.Millisecond)

	// Assert
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d during suppression, want 0", count)
	}
}

func TestWatcherTracksDirectoryWhilePaused(t *testing.T) {
	// Arrange
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

	subDir := filepath.Join(dir, "NewFolder")

	// Act - create directory during pause
	w.Suppress(func() {
		if err := os.Mkdir(subDir, 0o700); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
	})

	time.Sleep(300 * time.Millisecond)

	// Assert - creation was suppressed
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d during suppression, want 0", count)
	}

	// Act - write file to new directory after resume
	modFile := filepath.Join(subDir, "Mod_9999999_P.pak")
	if err := os.WriteFile(modFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Assert - file in newly tracked subfolder was detected
	if count := triggerCount.Load(); count == 0 {
		t.Fatal("triggerCount = 0 for file inside directory created while paused, want at least 1")
	}
}

func TestWatcherUntracksRemovedDirectory(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	subDir := filepath.Join(dir, "ToRemove")
	nestedDir := filepath.Join(subDir, "Nested")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

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

	// Act - remove parent directory and its nested children
	if err := os.RemoveAll(subDir); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Assert - directory removal triggered change
	if count := triggerCount.Load(); count == 0 {
		t.Fatal("triggerCount = 0 after directory removal, want at least 1")
	}

	w.mu.Lock()
	_, parentTracked := w.trackedDirs[subDir]
	_, nestedTracked := w.trackedDirs[nestedDir]
	w.mu.Unlock()

	if parentTracked {
		t.Errorf("subDir %q still tracked after removal", subDir)
	}
	if nestedTracked {
		t.Errorf("nestedDir %q still tracked after parent removal", nestedDir)
	}
}

func TestWatcherIgnoresEventsOutsideRoot(t *testing.T) {
	// Arrange
	dir1 := t.TempDir()
	dir2 := t.TempDir()

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

	if err := w.SetRoot(dir1); err != nil {
		t.Fatalf("SetRoot() error = %v", err)
	}

	// Act - deliver event for path outside root
	w.handleEvent(fsnotify.Event{
		Name: filepath.Join(dir2, "ExternalMod_P.pak"),
		Op:   fsnotify.Create,
	})

	time.Sleep(150 * time.Millisecond)

	// Assert
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d for event outside root, want 0", count)
	}
}
