package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

func TestRuntimeStatus(t *testing.T) {
	app := NewApp()

	const want = "Go backend connected"
	if got := app.RuntimeStatus(); got != want {
		t.Fatalf("RuntimeStatus() = %q, want %q", got, want)
	}
}

func TestScanLibrary(t *testing.T) {
	root := t.TempDir()
	primaryPath := filepath.Join(root, "Example_9999999_P.pak")
	if err := os.WriteFile(primaryPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	library, err := app.ScanLibrary(root)
	if err != nil {
		t.Fatal(err)
	}

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
