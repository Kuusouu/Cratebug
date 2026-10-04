package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/mutation"
)

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
