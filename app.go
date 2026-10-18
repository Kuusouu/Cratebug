package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/secret"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
	"github.com/Kuusouu/Cratebug/internal/watcher"
)

// Builds the shared core and the bound services, and handles the Wails
// lifecycle hooks. It is not bound itself; main.go binds the services.
type App struct {
	*core

	updates         *UpdateService
	signatureBypass *SignatureBypassService
	backups         *BackupService
	pakRepair       *PakRepairService
	organize        *OrganizeService
	library         *LibraryService
	settings        *SettingsService
	installs        *InstallService
	nexusMods       *NexusService
}

// Creates the application and its services.
func NewApp() (*App, error) {
	path, err := metadata.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve metadata storage location: %w", err)
	}
	keyPath, err := secret.DefaultNexusKeyPath()
	if err != nil {
		return nil, fmt.Errorf("resolve nexus key location: %w", err)
	}
	classifier := modtype.NewSessionClassifier(modtype.DefaultWorkerLauncher(nil))
	app := newApp(mutation.NewGameRunningChecker(), metadata.NewStore(path), classifier, nil, nil, secret.NewStore(keyPath, nexusKeyEntropy))
	if exe, exeErr := protocolExecutablePath(); exeErr == nil {
		app.nexusMods.protocol = urlscheme.New(urlscheme.SchemeNXM, exe)
	}
	app.nexusMods.allowProtocol = AppVersion != "dev" && looksLikeInstalledBuild()
	app.initWatcher()
	return app, nil
}

// Lets tests inject a deterministic game-running detector, a disposable
// metadata store, a custom classifier, an optional character table, an
// install session manager, and a secret store.
func newApp(
	gameRunningChecker mutation.GameRunningChecker,
	metadataStore metadata.Store,
	classifier *modtype.SessionClassifier,
	characterTable *modtype.CharacterTable,
	installSessionManager *install.SessionManager,
	secretStore secret.Store,
) *App {
	if classifier == nil {
		classifier = modtype.NewSessionClassifier(nil)
	}
	if installSessionManager == nil {
		installSessionManager = install.NewSessionManager()
	}
	c := &core{
		runtime:  &runtimeHost{},
		game:     gameRunningChecker,
		meta:     newMetadataGate(metadataStore),
		catalog:  newCatalog(classifier, characterTable),
		detector: gamedetect.NewDefaultRegistry(),
	}
	installs := newInstallService(c, installSessionManager)
	return &App{
		core:            c,
		updates:         newUpdateService(c),
		signatureBypass: newSignatureBypassService(c, defaultSigbypassToolkit()),
		backups:         newBackupService(c),
		pakRepair:       newPakRepairService(c),
		organize:        newOrganizeService(c),
		library:         newLibraryService(c),
		settings:        newSettingsService(c),
		installs:        installs,
		nexusMods:       newNexusService(c, installs, secretStore),
	}
}

func (a *App) initWatcher() {
	w, err := watcher.New(watcher.Options{
		Debounce: 300 * time.Millisecond,
		OnChange: func() {
			a.runtime.emit("library:fs-changed")
		},
	})
	if err == nil {
		a.watcher = w
	}
}

// startup is called by Wails when the application is launched, saving the runtime context.
func (a *App) startup(ctx context.Context) {
	a.runtime.set(ctx)
	a.nexusMods.startup()
	modRoot := a.meta.read().Settings.ModRoot
	if modRoot != "" && a.watcher != nil {
		_ = a.watcher.SetRoot(modRoot)
	}
}

// shutdown is called by Wails when the application is closing, ensuring
// any session-held workers or background resources are cleanly closed.
func (a *App) shutdown(_ context.Context) {
	if a.watcher != nil {
		_ = a.watcher.Close()
	}
	a.nexusMods.CancelNexusDownload()
	if a.catalog != nil {
		_ = a.catalog.close()
	}
	_ = a.installs.close()
}
