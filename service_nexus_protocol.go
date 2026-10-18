package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/nexus"
	"github.com/Kuusouu/Cratebug/internal/secret"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const uninstallExeName = "uninstall.exe"

// Per-user nxm:// handler state safe to show in Settings.
type NexusProtocolState struct {
	Ownership   string `json:"ownership"`
	OwnerName   string `json:"ownerName,omitempty"`
	OwnerPath   string `json:"ownerPath,omitempty"`
	UserChoice  bool   `json:"userChoice"`
	MachineWide bool   `json:"machineWide"`
	CanRegister bool   `json:"canRegister"`
	Enabled     bool   `json:"enabled"`
}

// Reports who owns nxm:// without writing the registry.
func (s *NexusService) NexusProtocolStatus() (NexusProtocolState, error) {
	registrar, err := s.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: s.allowProtocol}, nil
	}
	status, err := registrar.Status()
	if err != nil {
		return NexusProtocolState{CanRegister: s.allowProtocol}, err
	}
	return s.protocolState(status), nil
}

// Registers Cratebug as the per-user nxm:// handler. takeOver must be
// true when another application currently owns the scheme.
func (s *NexusService) RegisterNexusProtocol(takeOver bool) (NexusProtocolState, error) {
	if !s.allowProtocol {
		state, _ := s.NexusProtocolStatus()
		return state, fmt.Errorf("dev builds do not register as the nxm:// handler")
	}
	registrar, err := s.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: true}, err
	}
	displaced, err := registrar.Register(takeOver)
	if err != nil {
		state, _ := s.NexusProtocolStatus()
		return state, err
	}
	if err := s.persistHandlerEnabled(displaced); err != nil {
		if !displaced.Empty() {
			if rollbackErr := registrar.Unregister(displaced); rollbackErr != nil {
				return s.mustProtocolStatus(), fmt.Errorf("record previous nxm handler: %w (restore previous owner: %v)", err, rollbackErr)
			}
		}
		return s.mustProtocolStatus(), fmt.Errorf("record nxm handler: %w", err)
	}
	return s.mustProtocolStatus(), nil
}

// Restores the previous nxm:// handler when we own the scheme.
func (s *NexusService) UnregisterNexusProtocol() (NexusProtocolState, error) {
	registrar, err := s.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: s.allowProtocol}, err
	}
	// Persist opt-out before touching the registry so a later crash cannot
	// silently reclaim nxm:// on the next launch.
	if err := s.persistProtocolOptOut(true); err != nil {
		return s.mustProtocolStatus(), err
	}
	previous := s.meta.read().Settings.NexusProtocol
	if err := registrar.Unregister(urlscheme.Snapshot{
		Command:     previous.Command,
		Icon:        previous.Icon,
		Description: previous.Description,
		DesktopFile: previous.DesktopFile,
	}); err != nil {
		return s.mustProtocolStatus(), err
	}
	if err := s.persistProtocolSnapshot(urlscheme.Snapshot{}); err != nil {
		return s.mustProtocolStatus(), err
	}
	return s.mustProtocolStatus(), nil
}

// Claims nxm:// when the setting defaults on and nothing else owns it.
// Takeover of another app stays behind the Settings prompt.
func (s *NexusService) ensureNexusProtocol() {
	if !s.allowProtocol {
		return
	}
	if s.meta.read().Settings.NexusProtocolOptOut {
		return
	}
	registrar, err := s.protocolRegistrar()
	if err != nil {
		return
	}
	status, err := registrar.Status()
	if err != nil {
		return
	}
	switch status.Ownership {
	case urlscheme.OwnershipNone, urlscheme.OwnershipSelf, urlscheme.OwnershipSelfStale:
		_, _ = s.RegisterNexusProtocol(false)
	}
}

func (s *NexusService) protocolRegistrar() (*urlscheme.Registrar, error) {
	if s.protocol != nil {
		return s.protocol, nil
	}
	return nil, fmt.Errorf("protocol registrar is not available")
}

func (s *NexusService) protocolState(status urlscheme.Status) NexusProtocolState {
	ownerName := ""
	if status.OwnerPath != "" {
		ownerName = filepath.Base(status.OwnerPath)
	}
	return NexusProtocolState{
		Ownership:   string(status.Ownership),
		OwnerName:   ownerName,
		OwnerPath:   status.OwnerPath,
		UserChoice:  status.UserChoice,
		MachineWide: status.MachineWide,
		CanRegister: s.allowProtocol,
		Enabled:     status.Ownership == urlscheme.OwnershipSelf,
	}
}

func (s *NexusService) mustProtocolStatus() NexusProtocolState {
	state, err := s.NexusProtocolStatus()
	if err != nil {
		state.CanRegister = s.allowProtocol
	}
	return state
}

func (s *NexusService) persistProtocolSnapshot(snapshot urlscheme.Snapshot) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.SetNexusProtocol(metadata.NexusProtocolSnapshot{
			Command:     snapshot.Command,
			Icon:        snapshot.Icon,
			Description: snapshot.Description,
			DesktopFile: snapshot.DesktopFile,
		})
	})
}

func (s *NexusService) persistProtocolOptOut(optOut bool) error {
	return s.meta.update(func(doc *metadata.Document) error {
		doc.SetNexusProtocolOptOut(optOut)
		return nil
	})
}

func (s *NexusService) persistHandlerEnabled(displaced urlscheme.Snapshot) error {
	return s.meta.update(func(doc *metadata.Document) error {
		if !displaced.Empty() {
			if err := doc.SetNexusProtocol(metadata.NexusProtocolSnapshot{
				Command:     displaced.Command,
				Icon:        displaced.Icon,
				Description: displaced.Description,
				DesktopFile: displaced.DesktopFile,
			}); err != nil {
				return err
			}
		}
		doc.SetNexusProtocolOptOut(false)
		return nil
	})
}

func (s *NexusService) onSecondInstanceLaunch(data options.SecondInstanceData) {
	// Store and return. Network, registry, and staging belong on the
	// UI thread after TakePendingNexusLink, not on this callback.
	if raw := nexus.FirstDownloadURL(data.Args); raw != "" {
		s.setLaunchURL(raw)
	}
	s.pendingURLMu.Lock()
	ready := s.ctxReady
	s.pendingURLMu.Unlock()
	ctx := s.runtime.current()
	if !ready || ctx == nil {
		return
	}
	wailsRuntime.WindowUnminimise(ctx)
	wailsRuntime.WindowShow(ctx)
}

func runUninstallCleanup() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for uninstall cleanup: %w", err)
	}

	previous := metadata.NexusProtocolSnapshot{}
	if path, pathErr := metadata.DefaultPath(); pathErr == nil {
		doc, _ := metadata.NewStore(path).Load()
		previous = doc.Settings.NexusProtocol
	}

	registrar := urlscheme.New(urlscheme.SchemeNXM, exe)
	if err := registrar.Unregister(urlscheme.Snapshot{
		Command:     previous.Command,
		Icon:        previous.Icon,
		Description: previous.Description,
		DesktopFile: previous.DesktopFile,
	}); err != nil {
		return fmt.Errorf("restore nxm handler: %w", err)
	}

	keyPath, err := secret.DefaultNexusKeyPath()
	if err != nil {
		return fmt.Errorf("resolve nexus key location: %w", err)
	}
	if err := secret.NewStore(keyPath, nexusKeyEntropy).Clear(); err != nil {
		return fmt.Errorf("clear nexus API key: %w", err)
	}
	return nil
}

func looksLikeInstalledBuild() bool {
	if appImagePath() != "" {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), uninstallExeName))
	return err == nil
}

func protocolExecutablePath() (string, error) {
	if path := appImagePath(); path != "" {
		return path, nil
	}
	return os.Executable()
}

func appImagePath() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	path := os.Getenv("APPIMAGE")
	if !filepath.IsAbs(path) {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}
