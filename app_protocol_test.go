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
	if doc.Settings.NexusProtocolOptOut {
		t.Fatal("NexusProtocolOptOut = true after register, want false")
	}
}

func TestRegisterNexusProtocolRollsBackWhenSnapshotCannotBeSaved(t *testing.T) {
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
	foreign := urlscheme.Snapshot{
		Command:     `"` + otherExe + `" "%1"`,
		Icon:        `"` + otherExe + `"`,
		Description: "URL:cratebug-test-app Protocol",
	}
	if err := registrar.Unregister(foreign); err != nil {
		t.Fatal(err)
	}
	app.protocol = registrar
	app.metadataStore = failingMetadataStore(t)

	// Act
	_, err := app.RegisterNexusProtocol(true)

	// Assert
	if err == nil {
		t.Fatal("RegisterNexusProtocol() succeeded, want an error when the displaced owner cannot be recorded")
	}
	state, statusErr := app.NexusProtocolStatus()
	if statusErr != nil {
		t.Fatalf("NexusProtocolStatus() = %v", statusErr)
	}
	if state.Ownership != string(urlscheme.OwnershipOther) {
		t.Errorf("Ownership = %q, want other after a failed persist rolled back", state.Ownership)
	}
	if filepath.Base(state.OwnerPath) != "Vortex.exe" {
		t.Errorf("OwnerPath = %q, want Vortex.exe restored", state.OwnerPath)
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
	if !app.LoadMetadata().Document.Settings.NexusProtocolOptOut {
		t.Fatal("NexusProtocolOptOut = false after unregister, want true")
	}
}

func TestEnsureNexusProtocolRegistersWhenNothingOwnsTheScheme(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.allowProtocol = true
	app.protocol = urlscheme.NewForTest("cratebug-test-app", `C:\Apps\Cratebug\Cratebug.exe`, nil)

	// Act
	app.startup(t.Context())

	// Assert
	state, err := app.NexusProtocolStatus()
	if err != nil {
		t.Fatalf("NexusProtocolStatus() = %v", err)
	}
	if !state.Enabled {
		t.Fatal("Enabled = false after startup, want silent registration when nothing owns the scheme")
	}
}

func TestEnsureNexusProtocolDoesNotTakeOverAnotherOwner(t *testing.T) {
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
	app.startup(t.Context())

	// Assert
	state, err := app.NexusProtocolStatus()
	if err != nil {
		t.Fatalf("NexusProtocolStatus() = %v", err)
	}
	if state.Ownership != string(urlscheme.OwnershipOther) {
		t.Errorf("Ownership = %q, want other; silent startup must not take over", state.Ownership)
	}
}

func TestEnsureNexusProtocolSkipsAfterUnregister(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.allowProtocol = true
	app.protocol = urlscheme.NewForTest("cratebug-test-app", `C:\Apps\Cratebug\Cratebug.exe`, nil)
	if _, err := app.RegisterNexusProtocol(false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UnregisterNexusProtocol(); err != nil {
		t.Fatal(err)
	}

	// Act
	app.ensureNexusProtocol()

	// Assert
	state, err := app.NexusProtocolStatus()
	if err != nil {
		t.Fatalf("NexusProtocolStatus() = %v", err)
	}
	if state.Enabled {
		t.Fatal("Enabled = true after unregister, want the opt-out to block silent registration")
	}
}

func TestEnsureNexusProtocolSkipsDevBuilds(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.protocol = urlscheme.NewForTest("cratebug-test-app", `C:\Apps\Cratebug\Cratebug.exe`, nil)

	// Act
	app.ensureNexusProtocol()

	// Assert
	state, err := app.NexusProtocolStatus()
	if err != nil {
		t.Fatalf("NexusProtocolStatus() = %v", err)
	}
	if state.Enabled {
		t.Fatal("Enabled = true in a dev build, want no registration")
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
