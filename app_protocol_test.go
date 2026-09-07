package main

import (
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/urlscheme"
)

func TestRegisterNexusProtocolRejectedWhenNotAllowed(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.protocol = urlscheme.NewForTest("cratebug-test-app", `C:\Apps\Cratebug\Cratebug.exe`, nil)

	// Act
	_, err := app.RegisterNexusProtocol(false)

	// Assert
	if err == nil {
		t.Fatal("RegisterNexusProtocol() succeeded in a dev build, want an error")
	}
}

func TestRegisterNexusProtocolPersistsADisplacedOwner(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	app := testApp(t, false)
	app.allowProtocol = true
	registrar := urlscheme.NewForTest("cratebug-test-app", selfExe, func(path string) bool {
		return path == selfExe || path == otherExe
	})
	if _, err := registrar.Register(false); err != nil {
		t.Fatal(err)
	}
	// Pretend Vortex wrote the scheme first by taking over from ourselves
	// with a foreign snapshot, then re-pointing the registrar at Cratebug.
	foreign := urlscheme.Snapshot{
		Command:     `"` + otherExe + `" "%1"`,
		Icon:        `"` + otherExe + `"`,
		Description: "URL:cratebug-test-app Protocol",
	}
	if err := registrar.Unregister(foreign); err != nil {
		t.Fatal(err)
	}
	app.protocol = registrar

	// Act
	state, err := app.RegisterNexusProtocol(true)

	// Assert
	if err != nil {
		t.Fatalf("RegisterNexusProtocol() = %v", err)
	}
	if !state.Enabled {
		t.Fatal("Enabled = false after register, want true")
	}
	doc := app.LoadMetadata().Document
	if doc.Settings.NexusProtocol.Command != foreign.Command {
		t.Errorf("persisted command = %q, want the displaced Vortex command", doc.Settings.NexusProtocol.Command)
	}
}

func TestUnregisterNexusProtocolRestoresThePersistedOwner(t *testing.T) {
	// Arrange
	selfExe := `C:\Apps\Cratebug\Cratebug.exe`
	otherExe := `C:\Apps\Vortex\Vortex.exe`
	app := testApp(t, false)
	app.allowProtocol = true
	registrar := urlscheme.NewForTest("cratebug-test-app", selfExe, func(path string) bool {
		return path == selfExe || path == otherExe
	})
	app.protocol = registrar
	if _, err := app.RegisterNexusProtocol(false); err != nil {
		t.Fatal(err)
	}
	if err := app.persistProtocolSnapshot(urlscheme.Snapshot{
		Command:     `"` + otherExe + `" "%1"`,
		Icon:        `"` + otherExe + `"`,
		Description: "URL:cratebug-test-app Protocol",
	}); err != nil {
		t.Fatal(err)
	}

	// Act
	state, err := app.UnregisterNexusProtocol()

	// Assert
	if err != nil {
		t.Fatalf("UnregisterNexusProtocol() = %v", err)
	}
	if state.Enabled {
		t.Fatal("Enabled = true after unregister, want false")
	}
	if state.Ownership != string(urlscheme.OwnershipOther) {
		t.Errorf("Ownership = %q, want other", state.Ownership)
	}
	if filepath.Base(state.OwnerPath) != "Vortex.exe" {
		t.Errorf("OwnerName path = %q, want Vortex.exe", state.OwnerPath)
	}
}

func TestNexusProtocolStatusWithoutRegistrar(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	state, err := app.NexusProtocolStatus()

	// Assert
	if err != nil {
		t.Fatalf("NexusProtocolStatus() = %v, want a disabled state", err)
	}
	if state.CanRegister || state.Enabled {
		t.Fatalf("state = %+v, want CanRegister and Enabled false", state)
	}
}
