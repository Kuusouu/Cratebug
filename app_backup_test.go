package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/backup"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

func TestToolsWatcherSuspensionAllowsRestoreAndRescan(t *testing.T) {
	// Arrange
	source := t.TempDir()
	relative := filepath.Join("cnd", "pajama", "Restored_P.pak")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(source, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, relative), []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := backup.Create(context.Background(), source, "", zipPath, nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cnd", "pajama"), 0o700); err != nil {
		t.Fatal(err)
	}
	app := testApp(t, false)
	app.initWatcher()
	if app.watcher == nil {
		t.Fatal("library watcher is unavailable")
	}
	defer app.watcher.Close()
	if _, err := app.ScanLibrary(root); err != nil {
		t.Fatal(err)
	}
	preview, err := app.restoreSessions.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer app.restoreSessions.DiscardSession(preview.Token)

	// Act
	if err := app.SetLibraryWatcherSuspended(true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelled, err := suppressWatcherResult(app, func() (backup.RestoreResult, error) {
		return app.restoreSessions.Apply(ctx, root, "", preview.Token, func(backup.Progress) {
			cancel()
		})
	})

	// Assert
	if err != nil || !cancelled.Cancelled {
		t.Fatalf("restore cancellation = %+v, error = %v", cancelled, err)
	}
	if _, err := os.Stat(filepath.Join(root, "cnd", "pajama")); err != nil {
		t.Fatalf("cancelled restore did not return the original folder: %v", err)
	}

	// Act
	result, err := app.RestoreApply(root, preview.Token)
	if err != nil {
		t.Fatalf("restore with the Tools menu open: %v", err)
	}
	if _, err := app.ScanLibrary(root); err != nil {
		t.Fatal(err)
	}

	// Assert
	if result.Counts.Mods != 1 {
		t.Fatalf("restored mod count = %d, want 1", result.Counts.Mods)
	}
	contents, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil || string(contents) != "backup" {
		t.Fatalf("restored contents = %q, error = %v", contents, err)
	}
	child := filepath.Join(root, "cnd")
	parked := filepath.Join(t.TempDir(), "cnd")
	if err := os.Rename(child, parked); err != nil {
		t.Fatalf("restore or rescan resumed the watcher before menu dismissal: %v", err)
	}
	if err := os.Rename(parked, child); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := app.SetLibraryWatcherSuspended(false); err != nil {
		t.Fatal(err)
	}

	// Assert
	if app.watcher.Root() != root {
		t.Fatal("menu dismissal lost the selected library root")
	}
}

func TestRestoreApplyBlocksRunningGame(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := testApp(t, true)

	// Act
	_, err := app.RestoreApply(root, "missing-token")

	// Assert
	if !errors.Is(err, mutation.ErrGameRunning) {
		t.Fatalf("RestoreApply() error = %v, want ErrGameRunning", err)
	}
	if _, err := os.Lstat(primaryPath); err != nil {
		t.Errorf("library primary is missing: %v", err)
	}
}

func TestRestorePreviewRequiresRuntimeContext(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	_, err := app.RestorePreview()

	// Assert
	if err == nil {
		t.Fatalf("RestorePreview() error = nil, want a missing-context error")
	}
}

func TestBackupLibraryRequiresRuntimeContext(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	_, err := app.BackupLibrary(t.TempDir())

	// Assert
	if err == nil {
		t.Fatalf("BackupLibrary() error = nil, want a missing-context error")
	}
}
