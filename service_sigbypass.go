package main

import (
	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/sigbypass"
)

// Carries the pinned payload sources the bindings use. Tests inject
// synthetic files and payloads; production always uses the pinned list and
// the fetched release.
type sigbypassToolkit struct {
	InstalledFiles func() []sigbypass.PayloadFile
	Payload        func() (sigbypass.Payload, error)
}

// Signature bypass detection, install, and removal, bound to the frontend.
type SignatureBypassService struct {
	*core
	sigbypassTool sigbypassToolkit
}

func newSignatureBypassService(c *core, tool sigbypassToolkit) *SignatureBypassService {
	return &SignatureBypassService{core: c, sigbypassTool: tool}
}

func defaultSigbypassToolkit() sigbypassToolkit {
	return sigbypassToolkit{
		InstalledFiles: sigbypass.InstalledFiles,
		Payload:        sigbypass.PinnedPayload,
	}
}

// Reports the signature bypass state of the detected game installation, plus
// whether this build carries a verified payload to install. Read-only.
func (s *SignatureBypassService) SignatureBypassStatus() (sigbypass.Status, error) {
	payloadAvailable := false
	if payload, err := s.sigbypassTool.Payload(); err == nil {
		payloadAvailable = payload.Verify() == nil
	}

	detection, err := s.signatureBypassDetection()
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

	status, err := sigbypass.Inspect(gameDir, s.sigbypassTool.InstalledFiles())
	if err != nil {
		return sigbypass.Status{}, err
	}
	status.PayloadAvailable = payloadAvailable
	return status, nil
}

// Copies the verified bypass payload into the detected game installation.
// Blocked while Marvel Rivals runs; a file Cratebug does not own is never
// overwritten.
func (s *SignatureBypassService) InstallSignatureBypass() (sigbypass.Status, error) {
	if err := s.requireGameNotRunning(); err != nil {
		return sigbypass.Status{}, err
	}

	gameDir, err := s.signatureBypassGameDir()
	if err != nil {
		return sigbypass.Status{}, err
	}
	payload, err := s.sigbypassTool.Payload()
	if err != nil {
		return sigbypass.Status{}, err
	}
	if err := sigbypass.Install(gameDir, payload); err != nil {
		return sigbypass.Status{}, err
	}
	return s.SignatureBypassStatus()
}

// Removes the bypass payload from the detected game installation. Blocked
// while Marvel Rivals runs; a different file at a payload path refuses the
// removal.
func (s *SignatureBypassService) RemoveSignatureBypass() (sigbypass.Status, error) {
	if err := s.requireGameNotRunning(); err != nil {
		return sigbypass.Status{}, err
	}

	gameDir, err := s.signatureBypassGameDir()
	if err != nil {
		return sigbypass.Status{}, err
	}
	if err := sigbypass.Remove(gameDir, s.sigbypassTool.InstalledFiles()); err != nil {
		return sigbypass.Status{}, err
	}
	return s.SignatureBypassStatus()
}

// Detects the game installation through the provider the frontend library
// detection uses, defaulting to Steam when no provider was chosen yet.
func (s *SignatureBypassService) signatureBypassDetection() (gamedetect.Detection, error) {
	provider := s.meta.read().Settings.LibraryProvider
	if provider == "" {
		provider = gamedetect.ProviderSteam
	}
	return s.detector.Detect(provider)
}

func (s *SignatureBypassService) signatureBypassGameDir() (string, error) {
	detection, err := s.signatureBypassDetection()
	if err != nil {
		return "", err
	}
	if detection.State == gamedetect.StateNotFound {
		return "", sigbypass.ErrGameNotFound
	}
	return sigbypass.GameDirForPaks(detection.PaksPath)
}
