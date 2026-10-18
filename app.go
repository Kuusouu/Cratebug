package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/conflict"
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

	mutationExecutor      mutation.Executor
	installSessionManager *install.SessionManager
	updates               *UpdateService
	signatureBypass       *SignatureBypassService
	backups               *BackupService
	pakRepair             *PakRepairService

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
		mutationExecutor:      mutation.NewExecutor(gameRunningChecker),
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

// Releases library directory handles while the Tools menu is open.
func (a *App) SetLibraryWatcherSuspended(suspended bool) error {
	if a.watcher == nil {
		return nil
	}
	return a.watcher.SetSuspended(suspended)
}

// Confirms that the frontend can reach the Go application.
func (a *App) RuntimeStatus() string {
	return "Go backend connected"
}

// ClassificationType anchors modtype.Identity so Wails emits its TypeScript model into models.ts.
func (a *App) ClassificationType() modtype.Identity {
	return modtype.Identity{}
}

// Returns the read-only catalog discovered beneath modRoot.
func (a *App) ScanLibrary(modRoot string) (discovery.Library, error) {
	if a.watcher != nil {
		currentRoot := a.watcher.Root()
		targetRoot := strings.TrimSpace(modRoot)
		if targetRoot != "" {
			if abs, err := filepath.Abs(targetRoot); err == nil {
				targetRoot = abs
			}
		}
		if filepath.Clean(currentRoot) != filepath.Clean(targetRoot) {
			_ = a.watcher.SetRoot(modRoot)
		}
	}
	return discovery.Scan(modRoot)
}

// ClassifyLibrary determines the category and hero/skin identity for discovered entries.
func (a *App) ClassifyLibrary(modRoot string, entries []discovery.Entry) (map[string]modtype.Identity, error) {
	return a.catalog.classify(modRoot, entries)
}

// ConflictType anchors conflict.Result so Wails emits its TypeScript model into models.ts.
func (a *App) ConflictType() conflict.Result {
	return conflict.Result{}
}

// DetectConflicts scans entries for enabled mods that share internal Unreal
// asset paths. It classifies entries first (a no-op for anything already
// classified this session, since internal/modtype's SessionClassifier keyed
// on Classify's own mtime-based cache also skips the corresponding
// UAssetToolRivals call), then reuses each enabled mod's retained path
// listing from that same cache rather than resolving it a second time.
func (a *App) DetectConflicts(modRoot string, entries []discovery.Entry) (conflict.Result, error) {
	if _, err := a.catalog.classify(modRoot, entries); err != nil {
		return conflict.Result{}, err
	}

	paths := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if entry.State != discovery.StateEnabled {
			continue
		}
		if resolved, ok := a.catalog.pathsFor(modRoot, entry); ok {
			paths[entry.ID] = resolved
		}
	}

	return conflict.Detect(entries, paths), nil
}

// Changes one current scanner entry to the requested enabled state.
// The entry ID is scanner-issued, never an arbitrary filesystem path.
func (a *App) SetModEnabled(modRoot, entryID string, enabled bool) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewSetEnabledOperation(modRoot, entryID, enabled)
		return a.mutationExecutor.Execute(operation)
	})
}

// Renames one current scanner entry without exposing arbitrary filesystem paths.
func (a *App) RenameMod(modRoot, entryID, name string) (mutation.Result, error) {
	operation := mutation.NewRenameModOperation(modRoot, entryID, name)
	return a.executeAndReconcile(operation)
}

// Changes one current scanner entry's filename-based priority.
func (a *App) SetModPriority(modRoot, entryID string, priority int) (mutation.Result, error) {
	operation := mutation.NewSetPriorityOperation(modRoot, entryID, priority)
	return a.executeAndReconcile(operation)
}

// Moves one current scanner entry to an existing scanner-known folder.
func (a *App) MoveMod(modRoot, entryID, destinationFolder string) (mutation.Result, error) {
	operation := mutation.NewMoveModOperation(modRoot, entryID, destinationFolder)
	return a.executeAndReconcile(operation)
}

// Creates one folder beneath the root or an existing scanner-known folder.
func (a *App) CreateFolder(modRoot, parentFolder, name string) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewCreateFolderOperation(modRoot, parentFolder, name)
		return a.mutationExecutor.Execute(operation)
	})
}

// Renames one scanner-known physical folder.
func (a *App) RenameFolder(modRoot, folder, name string) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewRenameFolderOperation(modRoot, folder, name)
		return a.mutationExecutor.Execute(operation)
	})
}

// Moves one scanner-known physical folder beneath the root or another scanner-known folder.
func (a *App) MoveFolder(modRoot, folder, destinationParent string) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewMoveFolderOperation(modRoot, folder, destinationParent)
		return a.mutationExecutor.Execute(operation)
	})
}

// Deletes one current scanner entry through the Windows Recycle Bin.
func (a *App) DeleteMod(modRoot, entryID string, confirmed bool) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewDeleteModOperation(modRoot, entryID, confirmed)
		return a.mutationExecutor.Execute(operation)
	})
}

// Deletes one scanner-known physical folder and its contents through the Windows Recycle Bin.
func (a *App) DeleteFolder(modRoot, folder string, confirmed bool) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		operation := mutation.NewDeleteFolderOperation(modRoot, folder, confirmed)
		return a.mutationExecutor.Execute(operation)
	})
}

// Reports whether one scanner-known physical folder holds no entries at all.
func (a *App) IsFolderEmpty(modRoot, folder string) (bool, error) {
	return mutation.FolderIsEmpty(modRoot, folder)
}

// Executes operation, then re-points any persisted metadata (currently tag
// assignments) from its previous scanner ID to its new one so a rename,
// priority change, or move does not orphan a mod's metadata.
//
// A folder rename or move changes the scanner ID of every mod it contains,
// but its Result reports only the folder's own old and new paths, not a
// per-mod ID pair, so those operations call the executor directly instead of
// this helper. These operations do not reconcile metadata for mods in the folder.
func (a *App) executeAndReconcile(operation mutation.Operation) (mutation.Result, error) {
	return suppressWatcherResult(a.core, func() (mutation.Result, error) {
		result, err := a.mutationExecutor.Execute(operation)
		if err != nil {
			return result, err
		}
		if result.PreviousID == "" || result.PreviousID == result.ID {
			return result, nil
		}

		err = a.meta.update(func(doc *metadata.Document) error {
			if !doc.ReconcileMod(result.PreviousID, result.ID) {
				return errMetadataUnchanged
			}
			return nil
		})
		if err != nil {
			return result, fmt.Errorf("reconcile mod metadata: %w", err)
		}
		return result, nil
	})
}

// MetadataState is the persisted document plus whether it had to be
// recovered from a damaged file, so the frontend can surface that as
// actionable feedback instead of silently losing track of the event.
type MetadataState struct {
	Document       metadata.Document `json:"document"`
	Recovered      bool              `json:"recovered"`
	RecoveryReason string            `json:"recoveryReason,omitempty"`
}

// Returns the persisted settings, tag catalog, and per-mod tag assignments.
func (a *App) LoadMetadata() MetadataState {
	doc, recovery := a.meta.load()
	state := MetadataState{Document: doc, Recovered: recovery.Recovered}
	if recovery.Cause != nil {
		state.RecoveryReason = recovery.Cause.Error()
	}
	return state
}

// Persists the selected mod root so the library does not need to be
// reselected the next time Cratebug launches.
func (a *App) SetModRoot(modRoot string) error {
	if a.watcher != nil {
		_ = a.watcher.SetRoot(modRoot)
	}
	return a.meta.update(func(doc *metadata.Document) error {
		doc.Settings.ModRoot = modRoot
		return nil
	})
}

// DetectLibrary locates the named store provider's Marvel Rivals
// installation and mod library. Read-only: nothing is written for any
// detection outcome.
func (a *App) DetectLibrary(provider string) (gamedetect.Detection, error) {
	return a.detector.Detect(provider)
}

// CreateLibrary re-detects the named store provider's installation and
// creates its missing mod-library folder, the one write Cratebug performs
// outside a configured mod root. The frontend calls it only after the user
// confirmed the creation in a dialog.
func (a *App) CreateLibrary(provider string) (string, error) {
	return a.detector.CreateLibrary(provider)
}

// Persists which store provider library auto-detection targets.
func (a *App) SetLibraryProvider(provider string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.SetLibraryProvider(provider)
	})
}

// Persists the appearance theme (system, light, or dark).
func (a *App) SetTheme(theme string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.SetTheme(theme)
	})
}

// Persists the view mode Cratebug opens to on the next launch.
func (a *App) SetDefaultViewMode(mode string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.SetDefaultViewMode(mode)
	})
}

// Persists the accent choice: a preset id, a custom hex color, or empty for
// the default preset.
func (a *App) SetAccentColor(color string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.SetAccentColor(color)
	})
}

// Persists whether destructive confirmation dialogs skip their countdown.
func (a *App) SetSkipDestructiveDelay(skip bool) error {
	return a.meta.update(func(doc *metadata.Document) error {
		doc.SetSkipDestructiveDelay(skip)
		return nil
	})
}

// Adds a new tag to the persisted catalog.
func (a *App) CreateTag(name string) (metadata.Tag, error) {
	var tag metadata.Tag
	err := a.meta.update(func(doc *metadata.Document) error {
		created, err := doc.CreateTag(name)
		tag = created
		return err
	})
	if err != nil {
		return metadata.Tag{}, err
	}
	return tag, nil
}

// Renames an existing tag in the persisted catalog.
func (a *App) RenameTag(id, name string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.RenameTag(id, name)
	})
}

// Removes a tag from the catalog and every mod it was assigned to.
func (a *App) DeleteTag(id string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		return doc.DeleteTag(id)
	})
}

// Assigns an existing catalog tag to one current scanner entry, establishing
// the entry's persistent identity first if this is the first metadata
// recorded for it.
func (a *App) AssignModTag(entryID, tagID string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		modID, err := doc.EnsureMod(entryID)
		if err != nil {
			return err
		}
		return doc.AssignTag(modID, tagID)
	})
}

// Removes a tag assignment from one current scanner entry.
func (a *App) UnassignModTag(entryID, tagID string) error {
	return a.meta.update(func(doc *metadata.Document) error {
		modID, err := doc.EnsureMod(entryID)
		if err != nil {
			return err
		}
		return doc.UnassignTag(modID, tagID)
	})
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
