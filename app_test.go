package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/secret"
	"github.com/Kuusouu/Cratebug/internal/watcher"
)

type staticGameRunningChecker struct {
	gameRunning bool
}

func (checker staticGameRunningChecker) IsGameRunning() (bool, error) {
	return checker.gameRunning, nil
}

func testMetadataStore(t *testing.T) metadata.Store {
	t.Helper()
	return metadata.NewStore(filepath.Join(t.TempDir(), "metadata.json"))
}

func testSecretStore(t *testing.T) secret.Store {
	t.Helper()
	return secret.NewPlainStore(filepath.Join(t.TempDir(), "nexus.key"), []byte("test-entropy"))
}

func testApp(t *testing.T, gameRunning bool) *App {
	t.Helper()
	emptyTable := modtype.CharacterTable{}
	return newApp(staticGameRunningChecker{gameRunning: gameRunning}, testMetadataStore(t), nil, &emptyTable, nil, testSecretStore(t))
}

func testAppWithStore(t *testing.T, gameRunning bool, store metadata.Store) *App {
	t.Helper()
	emptyTable := modtype.CharacterTable{}
	return newApp(staticGameRunningChecker{gameRunning: gameRunning}, store, nil, &emptyTable, nil, testSecretStore(t))
}

func failingMetadataStore(t *testing.T) metadata.Store {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	return metadata.NewStore(filepath.Join(parent, "metadata.json"))
}

func TestRuntimeStatus(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	got := app.RuntimeStatus()

	// Assert
	const want = "Go backend connected"
	if got != want {
		t.Fatalf("RuntimeStatus() = %q, want %q", got, want)
	}
}

func TestAppShutdown(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act & Assert (shutdown must complete cleanly without panic)
	app.shutdown(context.Background())
}

func TestAppWatcherIntegration(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.initWatcher()
	if app.watcher == nil {
		t.Fatal("initWatcher() did not create watcher")
	}
	defer func() {
		if err := app.watcher.Close(); err != nil {
			t.Errorf("test cleanup: %v", err)
		}
	}()

	dir := t.TempDir()

	// Act
	_, scanErr := app.library.ScanLibrary(dir)
	gotRoot := app.watcher.Root()

	// Assert
	if scanErr != nil {
		t.Fatalf("ScanLibrary() error = %v", scanErr)
	}
	if gotRoot != dir {
		t.Errorf("watcher.Root() = %q, want %q", gotRoot, dir)
	}

	// Arrange - new directory for SetModRoot
	dir2 := t.TempDir()

	// Act
	setErr := app.library.SetModRoot(dir2)
	gotRoot2 := app.watcher.Root()

	// Assert
	if setErr != nil {
		t.Fatalf("SetModRoot() error = %v", setErr)
	}
	if gotRoot2 != dir2 {
		t.Errorf("watcher.Root() = %q, want %q", gotRoot2, dir2)
	}
}

func TestAppWatcherSuppressionAndTracking(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	var triggerCount atomic.Int32
	w, err := watcher.New(watcher.Options{
		Debounce: 50 * time.Millisecond,
		OnChange: func() {
			triggerCount.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("watcher.New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Errorf("test cleanup: %v", err)
		}
	}()
	app.watcher = w

	dir := t.TempDir()
	if _, err := app.library.ScanLibrary(dir); err != nil {
		t.Fatalf("ScanLibrary() error = %v", err)
	}

	// Act - create folder via internal app mutation
	if _, err := app.organize.CreateFolder(dir, "", "Subfolder"); err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	time.Sleep(350 * time.Millisecond)

	// Assert - mutation was suppressed
	if count := triggerCount.Load(); count != 0 {
		t.Fatalf("triggerCount = %d during mutation suppression, want 0", count)
	}

	// Act - external file written inside tracked subfolder
	subMod := filepath.Join(dir, "Subfolder", "External_9999999_P.pak")
	if err := os.WriteFile(subMod, []byte("pak content"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Assert - external change inside tracked subfolder triggered watcher
	if count := triggerCount.Load(); count == 0 {
		t.Fatal("triggerCount = 0 for external file in tracked subfolder, want at least 1")
	}
}

func TestAppWatcherScanLibraryNormalizesPaths(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.initWatcher()
	if app.watcher == nil {
		t.Fatal("initWatcher() did not create watcher")
	}
	defer func() {
		if err := app.watcher.Close(); err != nil {
			t.Errorf("test cleanup: %v", err)
		}
	}()

	dir := t.TempDir()
	if _, err := app.library.ScanLibrary(dir); err != nil {
		t.Fatalf("ScanLibrary() error = %v", err)
	}
	initialRoot := app.watcher.Root()

	// Act - scan again with trailing slash
	dirWithSlash := dir + string(filepath.Separator)
	if _, err := app.library.ScanLibrary(dirWithSlash); err != nil {
		t.Fatalf("ScanLibrary() error = %v", err)
	}
	subsequentRoot := app.watcher.Root()

	// Assert - watcher root is unchanged because paths are normalized
	if subsequentRoot != initialRoot {
		t.Errorf("watcher.Root() changed from %q to %q for equivalent path", initialRoot, subsequentRoot)
	}
}
