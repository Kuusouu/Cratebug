package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/nexus"
	"github.com/Kuusouu/Cratebug/internal/secret"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
	"github.com/Kuusouu/Cratebug/internal/watcher"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Exposes the small backend surface used by the frontend.
type App struct {
	*core

	installSessionManager *install.SessionManager
	updates               *UpdateService
	signatureBypass       *SignatureBypassService
	backups               *BackupService
	pakRepair             *PakRepairService
	organize              *OrganizeService
	library               *LibraryService
	settings              *SettingsService

	secretStore secret.Store

	pendingURLMu sync.Mutex
	pendingLink  *nexus.DownloadRequest
	linkSecrets  map[string]nexus.DownloadSecrets
	ctxReady     bool

	nexusMu             sync.Mutex
	nexusClient         *nexus.Client
	nexusDownloadCancel context.CancelFunc
	nexusBaseURL        string
	pendingNexusSource  *pendingNexusInstall

	protocol      *urlscheme.Registrar
	allowProtocol bool
}

// Creates the application binding.
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
		app.protocol = urlscheme.New(urlscheme.SchemeNXM, exe)
	}
	app.allowProtocol = AppVersion != "dev" && looksLikeInstalledBuild()
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
	return &App{
		core:                  c,
		updates:               newUpdateService(c),
		signatureBypass:       newSignatureBypassService(c, defaultSigbypassToolkit()),
		backups:               newBackupService(c),
		pakRepair:             newPakRepairService(c),
		organize:              newOrganizeService(c),
		library:               newLibraryService(c),
		settings:              newSettingsService(c),
		installSessionManager: installSessionManager,
		secretStore:           secretStore,
		linkSecrets:           make(map[string]nexus.DownloadSecrets),
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

// Confirms that the frontend can reach the Go application.
func (a *App) RuntimeStatus() string {
	return "Go backend connected"
}

// startup is called by Wails when the application is launched, saving the runtime context.
func (a *App) startup(ctx context.Context) {
	// The context is in place before ctxReady flips, so a link that
	// setLaunchURL sees as ready can always be emitted.
	a.runtime.set(ctx)
	a.pendingURLMu.Lock()
	a.ctxReady = true
	pending := a.pendingLink
	a.pendingURLMu.Unlock()
	if pending != nil {
		a.emitNexusLink(*pending)
	}
	a.ensureNexusProtocol()
	modRoot := a.meta.read().Settings.ModRoot
	if modRoot != "" && a.watcher != nil {
		_ = a.watcher.SetRoot(modRoot)
	}
}

// SelectFilesForInstall opens a native multiple-file dialog to select mod archives or direct bundles.
func (a *App) SelectFilesForInstall() ([]string, error) {
	ctx := a.runtime.current()
	if ctx == nil {
		return nil, fmt.Errorf("application runtime context is not available")
	}

	paths, err := wailsRuntime.OpenMultipleFilesDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Mods to Install",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Supported Mod Archives & Files (*.zip, *.7z, *.rar, *.pak, *.utoc, *.ucas)",
				Pattern:     "*.zip;*.7z;*.rar;*.tar;*.tar.gz;*.tgz;*.pak;*.utoc;*.ucas",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open file dialog: %w", err)
	}

	return paths, nil
}

// PrepareInstall creates a staging session, unpacks the selected files, discovers mods,
// and returns the preview information with collision checks.
func (a *App) PrepareInstall(modRoot string, filePaths []string, defaultFolder string) (install.PreviewResult, error) {
	if len(filePaths) == 0 {
		return install.PreviewResult{}, fmt.Errorf("no files selected")
	}
	a.clearPendingNexusSource()
	return a.stageAndPreview(modRoot, filePaths, defaultFolder)
}

// Stages filePaths into a fresh session and builds the install preview.
// Shared by PrepareInstall (local files) and PrepareNexusInstall (a
// downloaded Nexus file), which differ only in how filePaths' single
// entry was obtained.
func (a *App) stageAndPreview(modRoot string, filePaths []string, defaultFolder string) (install.PreviewResult, error) {
	session, err := a.installSessionManager.CreateSession(filePaths)
	if err != nil {
		return install.PreviewResult{}, fmt.Errorf("create staging session: %w", err)
	}

	onProgress := func(p install.Progress) {
		a.runtime.emit("install:progress", p)
	}

	if err := session.StageFiles(a.runtime.context(), onProgress); err != nil {
		_ = a.installSessionManager.RemoveSession(session.ID)
		return install.PreviewResult{}, err
	}

	var entries []discovery.Entry
	for _, mod := range session.Mods {
		entries = append(entries, discovery.Entry{
			ID:           mod.ID,
			Kind:         discovery.EntryMod,
			DisplayName:  mod.DisplayName,
			PrimaryPath:  mod.RelativePrimaryPath,
			BundleFormat: mod.BundleFormat,
			Sidecars:     mod.Sidecars,
		})
	}
	// Classification only enriches the preview with hero/category labels; a failure here
	// must not block installation, so the error is intentionally not propagated.
	identities, _ := a.catalog.classify(session.Dir, entries)

	if worker, err := uassettool.NewWriteWorker(nil); err == nil {
		markStagedCompanionPaks(session, worker)
		_ = worker.Close()
	}

	preview, err := install.BuildPreview(modRoot, session, defaultFolder, identities)
	if err != nil {
		_ = a.installSessionManager.RemoveSession(session.ID)
		return install.PreviewResult{}, err
	}

	return preview, nil
}

// ApplyInstall applies the approved installation items and cleans up the staging session.
func (a *App) ApplyInstall(modRoot string, sessionID string, items []install.ApplyItem) (install.ApplyResult, error) {
	return suppressWatcherResult(a.core, func() (install.ApplyResult, error) {
		session := a.installSessionManager.GetSession(sessionID)
		if session == nil {
			return install.ApplyResult{}, fmt.Errorf("install staging session %q not found or expired", sessionID)
		}
		defer func() {
			_ = a.installSessionManager.RemoveSession(sessionID)
		}()

		ctx := a.runtime.context()

		worker, err := uassettool.NewWriteWorker(nil)
		if err != nil {
			return install.ApplyResult{}, err
		}
		defer func() {
			_ = worker.Close() // Worker.Close waits for shutdown and always returns nil.
		}()
		onProgress := func(p install.Progress) {
			a.runtime.emit("install:progress", p)
		}
		if err := install.EncryptStagedMods(ctx, session, items, worker, onProgress); err != nil {
			return install.ApplyResult{}, err
		}
		if err := stripStagedCompanionPaks(session, items, worker); err != nil {
			return install.ApplyResult{}, err
		}

		onProgress(install.Progress{Phase: "applying", Message: "Install mods..."})
		result, err := install.Apply(ctx, modRoot, session, items, a.game)
		if err != nil {
			return result, err
		}

		if err := a.recordInstalledModMetadata(result); err != nil {
			// Files are already in the library. Wails rejects the JS promise when
			// the error is non-nil and discards ApplyResult, so the preview would
			// show a failed install and skip the library refresh.
			return result, nil
		}
		return result, nil
	})
}

func (a *App) recordInstalledModMetadata(result install.ApplyResult) error {
	source := a.takePendingNexusSource()
	err := a.meta.update(func(doc *metadata.Document) error {
		for _, entryID := range result.InstalledEntryIDs {
			modID, err := doc.EnsureMod(entryID)
			if err != nil {
				return fmt.Errorf("ensure installed mod metadata: %w", err)
			}
			if source != nil {
				if err := doc.SetModNexusSource(modID, source.ModID, source.FileID, source.Version); err != nil {
					return fmt.Errorf("record nexus source: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("save installed mod metadata: %w", err)
	}
	return nil
}

// CancelInstall cleans up staging data when the user cancels the installation preview.
func (a *App) CancelInstall(sessionID string) error {
	a.clearPendingNexusSource()
	return a.installSessionManager.RemoveSession(sessionID)
}

// shutdown is called by Wails when the application is closing, ensuring
// any session-held workers or background resources are cleanly closed.
func (a *App) shutdown(_ context.Context) {
	if a.watcher != nil {
		_ = a.watcher.Close()
	}
	a.CancelNexusDownload()
	if a.catalog != nil {
		_ = a.catalog.close()
	}
	if a.installSessionManager != nil {
		_ = a.installSessionManager.CleanupAll()
	}
}
