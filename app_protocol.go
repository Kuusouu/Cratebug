package main

import (
	"fmt"
	"os"
	"path/filepath"

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
func (a *App) NexusProtocolStatus() (NexusProtocolState, error) {
	registrar, err := a.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: a.allowProtocol}, nil
	}
	status, err := registrar.Status()
	if err != nil {
		return NexusProtocolState{CanRegister: a.allowProtocol}, err
	}
	return a.protocolState(status), nil
}

// Registers Cratebug as the per-user nxm:// handler. takeOver must be
// true when another application currently owns the scheme.
func (a *App) RegisterNexusProtocol(takeOver bool) (NexusProtocolState, error) {
	if !a.allowProtocol {
		state, _ := a.NexusProtocolStatus()
		return state, fmt.Errorf("dev builds do not register as the nxm:// handler")
	}
	registrar, err := a.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: true}, err
	}
	displaced, err := registrar.Register(takeOver)
	if err != nil {
		state, _ := a.NexusProtocolStatus()
		return state, err
	}
	if err := a.persistHandlerEnabled(displaced); err != nil {
		if !displaced.Empty() {
			if rollbackErr := registrar.Unregister(displaced); rollbackErr != nil {
				return a.mustProtocolStatus(), fmt.Errorf("record previous nxm handler: %w (restore previous owner: %v)", err, rollbackErr)
			}
		}
		return a.mustProtocolStatus(), fmt.Errorf("record nxm handler: %w", err)
	}
	return a.mustProtocolStatus(), nil
}

// Restores the previous nxm:// handler when we own the scheme.
func (a *App) UnregisterNexusProtocol() (NexusProtocolState, error) {
	registrar, err := a.protocolRegistrar()
	if err != nil {
		return NexusProtocolState{CanRegister: a.allowProtocol}, err
	}
	// Persist opt-out before touching the registry so a later crash cannot
	// silently reclaim nxm:// on the next launch.
	if err := a.persistProtocolOptOut(true); err != nil {
		return a.mustProtocolStatus(), err
	}
	previous := a.loadMetadataDocument().Settings.NexusProtocol
	if err := registrar.Unregister(urlscheme.Snapshot{
		Command:     previous.Command,
		Icon:        previous.Icon,
		Description: previous.Description,
	}); err != nil {
		return a.mustProtocolStatus(), err
	}
	if err := a.persistProtocolSnapshot(urlscheme.Snapshot{}); err != nil {
		return a.mustProtocolStatus(), err
	}
	return a.mustProtocolStatus(), nil
}

// Claims nxm:// when the setting defaults on and nothing else owns it.
// Takeover of another app stays behind the Settings prompt.
func (a *App) ensureNexusProtocol() {
	if !a.allowProtocol {
		return
	}
	if a.loadMetadataDocument().Settings.NexusProtocolOptOut {
		return
	}
	registrar, err := a.protocolRegistrar()
	if err != nil {
		return
	}
	status, err := registrar.Status()
	if err != nil {
		return
	}
	switch status.Ownership {
	case urlscheme.OwnershipNone, urlscheme.OwnershipSelf, urlscheme.OwnershipSelfStale:
		_, _ = a.RegisterNexusProtocol(false)
	}
}

func (a *App) protocolRegistrar() (*urlscheme.Registrar, error) {
	if a.protocol != nil {
		return a.protocol, nil
	}
	return nil, fmt.Errorf("protocol registrar is not available")
}

func (a *App) protocolState(status urlscheme.Status) NexusProtocolState {
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
		CanRegister: a.allowProtocol,
		Enabled:     status.Ownership == urlscheme.OwnershipSelf,
	}
}

func (a *App) mustProtocolStatus() NexusProtocolState {
	state, err := a.NexusProtocolStatus()
	if err != nil {
		state.CanRegister = a.allowProtocol
	}
	return state
}

func (a *App) persistProtocolSnapshot(snapshot urlscheme.Snapshot) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetNexusProtocol(metadata.NexusProtocolSnapshot{
		Command:     snapshot.Command,
		Icon:        snapshot.Icon,
		Description: snapshot.Description,
	}); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

func (a *App) persistProtocolOptOut(optOut bool) error {
	doc := a.loadMetadataDocument()
	doc.SetNexusProtocolOptOut(optOut)
	return a.metadataStore.Save(doc)
}

func (a *App) persistHandlerEnabled(displaced urlscheme.Snapshot) error {
	doc := a.loadMetadataDocument()
	if !displaced.Empty() {
		if err := doc.SetNexusProtocol(metadata.NexusProtocolSnapshot{
			Command:     displaced.Command,
			Icon:        displaced.Icon,
			Description: displaced.Description,
		}); err != nil {
			return err
		}
	}
	doc.SetNexusProtocolOptOut(false)
	return a.metadataStore.Save(doc)
}

func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	// Store and return. Network, registry, and staging belong on the
	// UI thread after TakePendingNexusLink, not on this callback.
	if raw := nexus.FirstDownloadURL(data.Args); raw != "" {
		a.setLaunchURL(raw)
	}
	a.pendingURLMu.Lock()
	ready := a.ctxReady
	ctx := a.ctx
	a.pendingURLMu.Unlock()
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
