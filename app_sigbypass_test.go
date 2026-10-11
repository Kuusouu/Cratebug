package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/sigbypass"
)

// Pins game detection to one outcome, registered under the Steam name that
// the default provider setting resolves to.
type staticSigbypassProvider struct {
	detection gamedetect.Detection
}

func (p staticSigbypassProvider) Name() string {
	return gamedetect.ProviderSteam
}

func (p staticSigbypassProvider) Detect() (gamedetect.Detection, error) {
	return p.detection, nil
}

// Creates the verified install shape and returns the Paks path detection
// would report.
func writeSigbypassGameInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	paksPath := filepath.Join(root, "MarvelGame", "Marvel", "Content", "Paks")
	win64 := filepath.Join(root, "MarvelGame", "Marvel", "Binaries", "Win64")
	if err := os.MkdirAll(paksPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(win64, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(win64, sigbypass.GameExecutableName), []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	return paksPath
}

// Makes a synthetic payload whose digests the app under test installs,
// standing in for the fetched release payload.
func writeSigbypassPayload(t *testing.T) sigbypass.Payload {
	t.Helper()
	dir := t.TempDir()
	loader := sigbypass.LoaderFileName
	plugin := filepath.Join(sigbypass.PluginDirName, sigbypass.PluginFileName)
	if err := os.MkdirAll(filepath.Join(dir, sigbypass.PluginDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, loader), []byte("loader-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plugin), []byte("plugin-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaderSum := sha256.Sum256([]byte("loader-bytes"))
	pluginSum := sha256.Sum256([]byte("plugin-bytes"))
	return sigbypass.Payload{
		Dir: dir,
		Files: []sigbypass.PayloadFile{
			{RelativePath: loader, SHA256: hex.EncodeToString(loaderSum[:])},
			{RelativePath: plugin, SHA256: hex.EncodeToString(pluginSum[:])},
		},
	}
}

func TestSignatureBypassInstallAndRemoveRoundTrip(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	paksPath := writeSigbypassGameInstall(t)
	payload := writeSigbypassPayload(t)
	app.detector = gamedetect.NewRegistry(staticSigbypassProvider{
		detection: gamedetect.Detection{State: gamedetect.StateLibraryFound, PaksPath: paksPath},
	})
	app.sigbypassTool = sigbypassToolkit{
		InstalledFiles: func() []sigbypass.PayloadFile { return payload.Files },
		Payload:        func() (sigbypass.Payload, error) { return payload, nil },
	}

	// Act
	installed, err := app.InstallSignatureBypass()

	// Assert
	if err != nil {
		t.Fatalf("InstallSignatureBypass() = %v", err)
	}
	if installed.State != sigbypass.StateInstalled || !installed.PayloadAvailable {
		t.Fatalf("installed status = %+v, want installed with an available payload", installed)
	}

	// Act
	removed, err := app.RemoveSignatureBypass()

	// Assert
	if err != nil {
		t.Fatalf("RemoveSignatureBypass() = %v", err)
	}
	if removed.State != sigbypass.StateNotInstalled {
		t.Fatalf("removed status = %+v, want notInstalled", removed)
	}
	win64 := filepath.Dir(filepath.Dir(paksPath))
	if _, statErr := os.Stat(filepath.Join(win64, "Binaries", "Win64", sigbypass.LoaderFileName)); !os.IsNotExist(statErr) {
		t.Error("loader still present after remove")
	}
}

func TestSignatureBypassMutationsAreBlockedWhileTheGameRuns(t *testing.T) {
	// Arrange
	app := testApp(t, true)

	// Act
	_, installErr := app.InstallSignatureBypass()
	_, removeErr := app.RemoveSignatureBypass()

	// Assert
	if !errors.Is(installErr, mutation.ErrGameRunning) {
		t.Errorf("InstallSignatureBypass() = %v, want ErrGameRunning", installErr)
	}
	if !errors.Is(removeErr, mutation.ErrGameRunning) {
		t.Errorf("RemoveSignatureBypass() = %v, want ErrGameRunning", removeErr)
	}
}

func TestSignatureBypassStatusReportsGameNotFoundWithoutAnInstall(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	payload := writeSigbypassPayload(t)
	app.detector = gamedetect.NewRegistry(staticSigbypassProvider{
		detection: gamedetect.Detection{State: gamedetect.StateNotFound},
	})
	app.sigbypassTool = sigbypassToolkit{
		InstalledFiles: func() []sigbypass.PayloadFile { return payload.Files },
		Payload:        func() (sigbypass.Payload, error) { return payload, nil },
	}

	// Act
	status, err := app.SignatureBypassStatus()

	// Assert
	if err != nil {
		t.Fatalf("SignatureBypassStatus() = %v", err)
	}
	if status.State != sigbypass.StateGameNotFound {
		t.Fatalf("State = %q, want %q", status.State, sigbypass.StateGameNotFound)
	}
	if !status.PayloadAvailable {
		t.Error("PayloadAvailable = false, want true")
	}
}

func TestSignatureBypassStatusReportsAMissingPayload(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	paksPath := writeSigbypassGameInstall(t)
	app.detector = gamedetect.NewRegistry(staticSigbypassProvider{
		detection: gamedetect.Detection{State: gamedetect.StateLibraryFound, PaksPath: paksPath},
	})
	app.sigbypassTool = sigbypassToolkit{
		InstalledFiles: sigbypass.InstalledFiles,
		Payload: func() (sigbypass.Payload, error) {
			return sigbypass.Payload{}, sigbypass.ErrPayloadNotFound
		},
	}

	// Act
	status, err := app.SignatureBypassStatus()

	// Assert
	if err != nil {
		t.Fatalf("SignatureBypassStatus() = %v", err)
	}
	if status.State != sigbypass.StateNotInstalled {
		t.Fatalf("State = %q, want %q", status.State, sigbypass.StateNotInstalled)
	}
	if status.PayloadAvailable {
		t.Error("PayloadAvailable = true, want false")
	}
}
