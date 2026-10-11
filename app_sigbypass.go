package main

import (
	"fmt"

	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/sigbypass"
)

// Carries the pinned payload sources the bindings use. Tests inject
// synthetic files and payloads; production always uses the pinned list and
// the fetched release.
type sigbypassToolkit struct {
	InstalledFiles func() []sigbypass.PayloadFile
	Payload        func() (sigbypass.Payload, error)
}

func defaultSigbypassToolkit() sigbypassToolkit {
	return sigbypassToolkit{
		InstalledFiles: sigbypass.InstalledFiles,
		Payload:        sigbypass.PinnedPayload,
	}
}

// Exists only so Wails emits the sigbypass.Status TypeScript model.
func (a *App) SignatureBypassStatusType() sigbypass.Status {
	return sigbypass.Status{}
}

// Reports the signature bypass state of the detected game installation, plus
// whether this build carries a verified payload to install. Read-only.
func (a *App) SignatureBypassStatus() (sigbypass.Status, error) {
	payloadAvailable := false
	if payload, err := a.sigbypassTool.Payload(); err == nil {
		payloadAvailable = payload.Verify() == nil
	}

	detection, err := a.signatureBypassDetection()
	if err != nil {
		return sigbypass.Status{}, err
	}
	if detection.State == gamedetect.StateNotFound {
		return sigbypass.Status{State: sigbypass.StateGameNotFound, PayloadAvailable: payloadAvailable}, nil
	}

	gameDir, err := sigbypass.GameDirForPaks(detection.PaksPath)
	if err != nil {
		// Detection found an installation but not the verified Win64 shape;
		// report the same state as an undetected game rather than a dead
		// card.
		return sigbypass.Status{State: sigbypass.StateGameNotFound, PayloadAvailable: payloadAvailable}, nil
	}

	status, err := sigbypass.Inspect(gameDir, a.sigbypassTool.InstalledFiles())
	if err != nil {
		return sigbypass.Status{}, err
	}
	status.PayloadAvailable = payloadAvailable
	return status, nil
}

// Copies the verified bypass payload into the detected game installation.
// Blocked while Marvel Rivals runs; a file Cratebug does not own is never
// overwritten.
func (a *App) InstallSignatureBypass() (sigbypass.Status, error) {
	if err := a.requireGameNotRunning(); err != nil {
		return sigbypass.Status{}, err
	}

	gameDir, err := a.signatureBypassGameDir()
	if err != nil {
		return sigbypass.Status{}, err
	}
	payload, err := a.sigbypassTool.Payload()
	if err != nil {
		return sigbypass.Status{}, err
	}
	if err := sigbypass.Install(gameDir, payload); err != nil {
		return sigbypass.Status{}, err
	}
	return a.SignatureBypassStatus()
}

// Removes the bypass payload from the detected game installation. Blocked
// while Marvel Rivals runs; a different file at a payload path refuses the
// removal.
func (a *App) RemoveSignatureBypass() (sigbypass.Status, error) {
	if err := a.requireGameNotRunning(); err != nil {
		return sigbypass.Status{}, err
	}

	gameDir, err := a.signatureBypassGameDir()
	if err != nil {
		return sigbypass.Status{}, err
	}
	if err := sigbypass.Remove(gameDir, a.sigbypassTool.InstalledFiles()); err != nil {
		return sigbypass.Status{}, err
	}
	return a.SignatureBypassStatus()
}

func (a *App) requireGameNotRunning() error {
	if a.gameRunningChecker == nil {
		return nil
	}
	running, err := a.gameRunningChecker.IsGameRunning()
	if err != nil {
		return fmt.Errorf("check whether Marvel Rivals is running: %w", err)
	}
	if running {
		return mutation.ErrGameRunning
	}
	return nil
}

// Detects the game installation through the provider the frontend library
// detection uses, defaulting to Steam when no provider was chosen yet.
func (a *App) signatureBypassDetection() (gamedetect.Detection, error) {
	provider := a.loadMetadataDocument().Settings.LibraryProvider
	if provider == "" {
		provider = gamedetect.ProviderSteam
	}
	return a.detector.Detect(provider)
}

func (a *App) signatureBypassGameDir() (string, error) {
	detection, err := a.signatureBypassDetection()
	if err != nil {
		return "", err
	}
	if detection.State == gamedetect.StateNotFound {
		return "", sigbypass.ErrGameNotFound
	}
	return sigbypass.GameDirForPaks(detection.PaksPath)
}
