package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/conflict"
	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/update"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The GitHub repository release.yml publishes to and CheckForUpdate reads from.
const (
	updateRepoOwner = "Kuusouu"
	updateRepoName  = "Cratebug"

	// Deliberately distinct from internal/update's own applyStagingDirName:
	// writeApplyScript clears its staging directory with os.RemoveAll before
	// writing the helper script, which would delete the downloaded installer
	// if the two shared a directory.
	updateDownloadDirName = "cratebug-update-download"
)

// Exposes the small backend surface used by the frontend.
type App struct {
	ctx                   context.Context
	gameRunningChecker    mutation.GameRunningChecker
	mutationExecutor      mutation.Executor
	metadataStore         metadata.Store
	classifier            *modtype.SessionClassifier
	installSessionManager *install.SessionManager
	tableMu               sync.Mutex
	characterTable        modtype.CharacterTable
	tableLoaded           bool
}

// Creates the application binding.
func NewApp() (*App, error) {
	path, err := metadata.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve metadata storage location: %w", err)
	}
	classifier := modtype.NewSessionClassifier(modtype.DefaultWorkerLauncher(nil))
	return newApp(mutation.WindowsGameRunningChecker{}, metadata.NewStore(path), classifier, nil, nil), nil
}

// Lets tests inject a deterministic game-running detector, a disposable
// metadata store, a custom classifier, an optional character table, and an install session manager.
func newApp(
	gameRunningChecker mutation.GameRunningChecker,
	metadataStore metadata.Store,
	classifier *modtype.SessionClassifier,
	characterTable *modtype.CharacterTable,
	installSessionManager *install.SessionManager,
) *App {
	if classifier == nil {
		classifier = modtype.NewSessionClassifier(nil)
	}
	if installSessionManager == nil {
		installSessionManager = install.NewSessionManager()
	}
	app := &App{
		gameRunningChecker:    gameRunningChecker,
		mutationExecutor:      mutation.NewExecutor(gameRunningChecker),
		metadataStore:         metadataStore,
		classifier:            classifier,
		installSessionManager: installSessionManager,
	}
	if characterTable != nil {
		app.characterTable = *characterTable
		app.tableLoaded = true
	}
	return app
}

// Confirms that the frontend can reach the Go application.
func (a *App) RuntimeStatus() string {
	return "Go backend connected"
}

// GetAppVersion returns the running build's CalVer release tag, or "dev" for
// a local or CI build, for display in Settings.
func (a *App) GetAppVersion() string {
	return AppVersion
}

// ClassificationType anchors modtype.Identity so Wails emits its TypeScript model into models.ts.
func (a *App) ClassificationType() modtype.Identity {
	return modtype.Identity{}
}

// Returns the read-only catalog discovered beneath modRoot.
func (a *App) ScanLibrary(modRoot string) (discovery.Library, error) {
	return discovery.Scan(modRoot)
}

// ClassifyLibrary determines the category and hero/skin identity for discovered entries.
func (a *App) ClassifyLibrary(modRoot string, entries []discovery.Entry) (map[string]modtype.Identity, error) {
	table := a.getCharacterTable()
	return a.classifier.Classify(modRoot, entries, table)
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
	table := a.getCharacterTable()
	if _, err := a.classifier.Classify(modRoot, entries, table); err != nil {
		return conflict.Result{}, err
	}

	paths := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if entry.State != discovery.StateEnabled {
			continue
		}
		if resolved, ok := a.classifier.PathsForEntry(modRoot, entry); ok {
			paths[entry.ID] = resolved
		}
	}

	return conflict.Detect(entries, paths), nil
}

func (a *App) getCharacterTable() modtype.CharacterTable {
	a.tableMu.Lock()
	defer a.tableMu.Unlock()

	if a.tableLoaded {
		return a.characterTable
	}

	cachePath, _ := modtype.DefaultCharacterTableCachePath()
	table := modtype.LoadCharacterTable(context.Background(), cachePath, modtype.DefaultCharacterTableMaxAge)
	a.characterTable = table
	a.tableLoaded = true
	return a.characterTable
}

// UpdateType anchors update.Release so Wails emits its TypeScript model into models.ts.
func (a *App) UpdateType() update.Release {
	return update.Release{}
}

// Reports whether a newer Cratebug release is published, and its details when it is.
type UpdateCheckResult struct {
	Available bool           `json:"available"`
	Release   update.Release `json:"release,omitempty"`
}

// Reports download progress for an in-progress update download, emitted on
// the "update:downloadProgress" event as it downloads (matching
// PrepareInstall's "install:progress" event for the same purpose).
type UpdateDownloadProgress struct {
	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
}

// CheckForUpdate compares the running build against the latest published
// GitHub release. A local or CI build (AppVersion is "dev", not a real
// release tag) never reports one available, since there is nothing valid to
// compare against.
func (a *App) CheckForUpdate() (UpdateCheckResult, error) {
	current, err := update.ParseVersion(AppVersion)
	if err != nil {
		return UpdateCheckResult{}, nil
	}

	release, err := a.updateClient().Latest(a.updateContext())
	if err != nil {
		if errors.Is(err, update.ErrNoRelease) {
			return UpdateCheckResult{}, nil
		}
		return UpdateCheckResult{}, err
	}

	if !release.Version.IsNewer(current) {
		return UpdateCheckResult{}, nil
	}
	return UpdateCheckResult{Available: true, Release: release}, nil
}

// CheckWhatsNew reports the changelog for the running build's own release,
// once, the first time it runs after that build changed -- the "what's new"
// notice shown right after an update relaunches Cratebug, not a check for a
// further update. Persists the running version as seen immediately so a
// crash before the frontend renders the notice can only lose it, never
// repeat it on every subsequent launch.
func (a *App) CheckWhatsNew() (UpdateCheckResult, error) {
	current, err := update.ParseVersion(AppVersion)
	if err != nil {
		return UpdateCheckResult{}, nil
	}

	doc := a.loadMetadataDocument()
	if doc.Settings.LastSeenVersion == current.Tag {
		return UpdateCheckResult{}, nil
	}

	release, err := a.updateClient().Tag(a.updateContext(), current.Tag)
	if err != nil {
		// Best-effort: a missing or unreachable release for the running tag
		// (offline, or a build that was never actually published) should not
		// block startup or repeat every launch, so it's still marked seen.
		doc.SetLastSeenVersion(current.Tag)
		_ = a.metadataStore.Save(doc)
		return UpdateCheckResult{}, nil
	}

	doc.SetLastSeenVersion(current.Tag)
	if saveErr := a.metadataStore.Save(doc); saveErr != nil {
		return UpdateCheckResult{}, fmt.Errorf("record the shown changelog version: %w", saveErr)
	}
	return UpdateCheckResult{Available: true, Release: release}, nil
}

// DownloadUpdate downloads release's installer, reporting progress through
// the "update:downloadProgress" event. Returns the downloaded installer's
// local path for a subsequent ApplyUpdate call.
func (a *App) DownloadUpdate(release update.Release) (string, error) {
	destDir := filepath.Join(os.TempDir(), updateDownloadDirName)
	onProgress := func(downloaded, total int64) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "update:downloadProgress", UpdateDownloadProgress{
				Downloaded: downloaded,
				Total:      total,
			})
		}
	}
	return a.updateClient().Download(a.updateContext(), release.Asset, destDir, onProgress)
}

// ApplyUpdate launches the detached helper that installs the update
// downloaded to installerPath and relaunches Cratebug, then quits the
// running instance so the helper can replace it. A nil error means Cratebug
// is about to exit, not that the update finished applying -- that happens in
// the detached helper after this process is gone.
func (a *App) ApplyUpdate(installerPath string) error {
	if err := update.ApplyUpdate(installerPath); err != nil {
		return err
	}
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
	return nil
}

func (a *App) updateClient() *update.Client {
	return &update.Client{Owner: updateRepoOwner, Repo: updateRepoName}
}

func (a *App) updateContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// Changes one current scanner entry to the requested enabled state.
// The entry ID is scanner-issued, never an arbitrary filesystem path.
func (a *App) SetModEnabled(modRoot, entryID string, enabled bool) (mutation.Result, error) {
	operation := mutation.NewSetEnabledOperation(modRoot, entryID, enabled)
	return a.mutationExecutor.Execute(operation)
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
	operation := mutation.NewCreateFolderOperation(modRoot, parentFolder, name)
	return a.mutationExecutor.Execute(operation)
}

// Renames one scanner-known physical folder.
func (a *App) RenameFolder(modRoot, folder, name string) (mutation.Result, error) {
	operation := mutation.NewRenameFolderOperation(modRoot, folder, name)
	return a.mutationExecutor.Execute(operation)
}

// Moves one scanner-known physical folder beneath the root or another scanner-known folder.
func (a *App) MoveFolder(modRoot, folder, destinationParent string) (mutation.Result, error) {
	operation := mutation.NewMoveFolderOperation(modRoot, folder, destinationParent)
	return a.mutationExecutor.Execute(operation)
}

// Deletes one current scanner entry through the Windows Recycle Bin.
func (a *App) DeleteMod(modRoot, entryID string, confirmed bool) (mutation.Result, error) {
	operation := mutation.NewDeleteModOperation(modRoot, entryID, confirmed)
	return a.mutationExecutor.Execute(operation)
}

// Executes operation, then re-points any persisted metadata (currently tag
// assignments) from its previous scanner ID to its new one so a rename,
// priority change, or move does not orphan a mod's metadata.
//
// A folder rename or move changes the scanner ID of every mod it contains,
// but its Result reports only the folder's own old and new paths, not a
// per-mod ID pair, so those operations call the executor directly instead of
// this helper. Metadata for mods inside a renamed or moved folder is not
// reconciled; this matches the existing frontend limitation described in
// docs/reviews/phase-4-review.md.
func (a *App) executeAndReconcile(operation mutation.Operation) (mutation.Result, error) {
	result, err := a.mutationExecutor.Execute(operation)
	if err != nil {
		return result, err
	}
	if result.PreviousID == "" || result.PreviousID == result.ID {
		return result, nil
	}

	doc := a.loadMetadataDocument()
	if !doc.ReconcileMod(result.PreviousID, result.ID) {
		return result, nil
	}
	if err := a.metadataStore.Save(doc); err != nil {
		return result, fmt.Errorf("reconcile mod metadata: %w", err)
	}
	return result, nil
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
	doc, recovery := a.metadataStore.Load()
	state := MetadataState{Document: doc, Recovered: recovery.Recovered}
	if recovery.Cause != nil {
		state.RecoveryReason = recovery.Cause.Error()
	}
	return state
}

// Loads the persisted document for an operation that only needs to read or
// mutate it, discarding recovery details that LoadMetadata reports instead.
func (a *App) loadMetadataDocument() metadata.Document {
	doc, _ := a.metadataStore.Load()
	return doc
}

// Persists the selected mod root so the library does not need to be
// reselected the next time Cratebug launches.
func (a *App) SetModRoot(modRoot string) error {
	doc := a.loadMetadataDocument()
	doc.Settings.ModRoot = modRoot
	return a.metadataStore.Save(doc)
}

// Persists the appearance theme (system, light, or dark).
func (a *App) SetTheme(theme string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetTheme(theme); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Persists the view mode Cratebug opens to on the next launch.
func (a *App) SetDefaultViewMode(mode string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetDefaultViewMode(mode); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Persists the accent color override, or clears it if color is empty.
func (a *App) SetAccentColor(color string) error {
	doc := a.loadMetadataDocument()
	if err := doc.SetAccentColor(color); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Adds a new tag to the persisted catalog.
func (a *App) CreateTag(name string) (metadata.Tag, error) {
	doc := a.loadMetadataDocument()
	tag, err := doc.CreateTag(name)
	if err != nil {
		return metadata.Tag{}, err
	}
	if err := a.metadataStore.Save(doc); err != nil {
		return metadata.Tag{}, err
	}
	return tag, nil
}

// Renames an existing tag in the persisted catalog.
func (a *App) RenameTag(id, name string) error {
	doc := a.loadMetadataDocument()
	if err := doc.RenameTag(id, name); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Removes a tag from the catalog and every mod it was assigned to.
func (a *App) DeleteTag(id string) error {
	doc := a.loadMetadataDocument()
	if err := doc.DeleteTag(id); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Assigns an existing catalog tag to one current scanner entry, establishing
// the entry's persistent identity first if this is the first metadata
// recorded for it.
func (a *App) AssignModTag(entryID, tagID string) error {
	doc := a.loadMetadataDocument()
	modID, err := doc.EnsureMod(entryID)
	if err != nil {
		return err
	}
	if err := doc.AssignTag(modID, tagID); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// Removes a tag assignment from one current scanner entry.
func (a *App) UnassignModTag(entryID, tagID string) error {
	doc := a.loadMetadataDocument()
	modID, err := doc.EnsureMod(entryID)
	if err != nil {
		return err
	}
	if err := doc.UnassignTag(modID, tagID); err != nil {
		return err
	}
	return a.metadataStore.Save(doc)
}

// startup is called by Wails when the application is launched, saving the runtime context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectFilesForInstall opens a native multiple-file dialog to select mod archives or direct bundles.
func (a *App) SelectFilesForInstall() ([]string, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("application runtime context is not available")
	}

	paths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
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
	return a.stageAndPreview(modRoot, filePaths, defaultFolder)
}

// InstallFromURL downloads a mod archive or bundle from rawURL, then runs it
// through the exact same staging and preview flow as a locally-selected
// file: the download is the only step that differs from PrepareInstall,
// matching SPEC.md's requirement that a remote download carry no less
// scrutiny than a local one. The downloaded temp file is removed once
// staging has copied out of it, regardless of whether staging succeeds.
func (a *App) InstallFromURL(modRoot, rawURL, defaultFolder string) (install.PreviewResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	onProgress := func(p install.Progress) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "install:progress", p)
		}
	}

	downloadedPath, cleanup, err := install.DownloadRemoteFile(ctx, rawURL, nil, onProgress)
	if err != nil {
		return install.PreviewResult{}, err
	}
	defer cleanup()

	return a.stageAndPreview(modRoot, []string{downloadedPath}, defaultFolder)
}

// Stages filePaths into a fresh session and builds the install preview.
// Shared by PrepareInstall (local files) and InstallFromURL (a downloaded
// file), which differ only in how filePaths' single entry was obtained.
func (a *App) stageAndPreview(modRoot string, filePaths []string, defaultFolder string) (install.PreviewResult, error) {
	session, err := a.installSessionManager.CreateSession(filePaths)
	if err != nil {
		return install.PreviewResult{}, fmt.Errorf("create staging session: %w", err)
	}

	onProgress := func(p install.Progress) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "install:progress", p)
		}
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if err := session.StageFiles(ctx, onProgress); err != nil {
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
	identities, _ := a.classifier.Classify(session.Dir, entries, a.getCharacterTable())

	preview, err := install.BuildPreview(modRoot, session, defaultFolder, identities)
	if err != nil {
		_ = a.installSessionManager.RemoveSession(session.ID)
		return install.PreviewResult{}, err
	}

	return preview, nil
}

// ApplyInstall applies the approved installation items and cleans up the staging session.
func (a *App) ApplyInstall(modRoot string, sessionID string, items []install.ApplyItem) (install.ApplyResult, error) {
	session := a.installSessionManager.GetSession(sessionID)
	if session == nil {
		return install.ApplyResult{}, fmt.Errorf("install staging session %q not found or expired", sessionID)
	}
	defer func() {
		_ = a.installSessionManager.RemoveSession(sessionID)
	}()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	return install.Apply(ctx, modRoot, session, items, a.gameRunningChecker)
}

// CancelInstall cleans up staging data when the user cancels the installation preview.
func (a *App) CancelInstall(sessionID string) error {
	return a.installSessionManager.RemoveSession(sessionID)
}

// shutdown is called by Wails when the application is closing, ensuring
// any session-held workers or background resources are cleanly closed.
func (a *App) shutdown(_ context.Context) {
	if a.classifier != nil {
		_ = a.classifier.Close()
	}
	if a.installSessionManager != nil {
		_ = a.installSessionManager.CleanupAll()
	}
}
