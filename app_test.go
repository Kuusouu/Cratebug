package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

type staticGameRunningChecker struct {
	gameRunning bool
}

func (checker staticGameRunningChecker) IsGameRunning() (bool, error) {
	return checker.gameRunning, nil
}

func TestRuntimeStatus(t *testing.T) {
	// Arrange
	app := newApp(staticGameRunningChecker{})

	// Act
	got := app.RuntimeStatus()

	// Assert
	const want = "Go backend connected"
	if got != want {
		t.Fatalf("RuntimeStatus() = %q, want %q", got, want)
	}
}

func TestSetModEnabledBlocksRunningGame(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := newApp(staticGameRunningChecker{gameRunning: true})

	// Act
	_, err := app.SetModEnabled(root, "Example_9999999_P.pak", false)

	// Assert
	if !errors.Is(err, mutation.ErrGameRunning) {
		t.Fatalf("SetModEnabled() error = %v, want ErrGameRunning", err)
	}
	if _, err := os.Lstat(primaryPath); err != nil {
		t.Errorf("enabled primary is missing: %v", err)
	}
}

func TestScanLibrary(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := newApp(staticGameRunningChecker{})

	// Act
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if library.Root != root {
		t.Errorf("Root = %q, want %q", library.Root, root)
	}
	if len(library.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(library.Entries))
	}
	entry := library.Entries[0]
	if entry.DisplayName != "Example" {
		t.Errorf("DisplayName = %q, want %q", entry.DisplayName, "Example")
	}
	if entry.State != discovery.StateEnabled {
		t.Errorf("State = %q, want %q", entry.State, discovery.StateEnabled)
	}
}

func TestSetModEnabled(t *testing.T) {
	// Arrange
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.bak_bento")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := newApp(staticGameRunningChecker{})

	// Act
	result, err := app.SetModEnabled(root, "Example_9999999_P.bak_bento", true)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if result.PrimaryPath != "Example_9999999_P.pak" || result.State != discovery.StateEnabled {
		t.Errorf("result = %#v, want enabled Example_9999999_P.pak", result)
	}
}
