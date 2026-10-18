package main

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Staging, preview, and apply for local files and Nexus downloads, bound to
// the frontend.
type InstallService struct {
	*core
	sessions *install.SessionManager

	// The Nexus file each staged session was downloaded from, keyed by
	// session ID, so ApplyInstall records the source of that session's mods
	// and no other.
	sourcesMu sync.Mutex
	sources   map[string]nexusSource
}

// Identifies the Nexus file a staged install came from.
type nexusSource struct {
	ModID   int
	FileID  int
	Version string
}

type companionPakCaller interface {
	Call(action string, params map[string]any, result any) error
}

func newInstallService(c *core, sessions *install.SessionManager) *InstallService {
	return &InstallService{
		core:     c,
		sessions: sessions,
		sources:  make(map[string]nexusSource),
	}
}

// SelectFilesForInstall opens a native multiple-file dialog to select mod archives or direct bundles.
func (s *InstallService) SelectFilesForInstall() ([]string, error) {
	ctx := s.runtime.current()
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
func (s *InstallService) PrepareInstall(modRoot string, filePaths []string, defaultFolder string) (install.PreviewResult, error) {
	if len(filePaths) == 0 {
		return install.PreviewResult{}, fmt.Errorf("no files selected")
	}
	return s.stage(modRoot, filePaths, defaultFolder, nil)
}

// Stages filePaths into a fresh session and builds the install preview.
// Shared by PrepareInstall (local files, no source) and
// NexusService.PrepareNexusInstall (a downloaded Nexus file, with its
// source recorded against the new session).
func (s *InstallService) stage(modRoot string, filePaths []string, defaultFolder string, source *nexusSource) (install.PreviewResult, error) {
	session, err := s.sessions.CreateSession(filePaths)
	if err != nil {
		return install.PreviewResult{}, fmt.Errorf("create staging session: %w", err)
	}

	onProgress := func(p install.Progress) {
		s.runtime.emit("install:progress", p)
	}

	if err := session.StageFiles(s.runtime.context(), onProgress); err != nil {
		_ = s.sessions.RemoveSession(session.ID)
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
	identities, _ := s.catalog.classify(session.Dir, entries)

	if worker, err := uassettool.NewWriteWorker(nil); err == nil {
		markStagedCompanionPaks(session, worker)
		_ = worker.Close()
	}

	preview, err := install.BuildPreview(modRoot, session, defaultFolder, identities)
	if err != nil {
		_ = s.sessions.RemoveSession(session.ID)
		return install.PreviewResult{}, err
	}

	if source != nil {
		s.sourcesMu.Lock()
		s.sources[session.ID] = *source
		s.sourcesMu.Unlock()
	}
	return preview, nil
}

// ApplyInstall applies the approved installation items and cleans up the staging session.
func (s *InstallService) ApplyInstall(modRoot string, sessionID string, items []install.ApplyItem) (install.ApplyResult, error) {
	return suppressWatcherResult(s.core, func() (install.ApplyResult, error) {
		source := s.takeSource(sessionID)
		session := s.sessions.GetSession(sessionID)
		if session == nil {
			return install.ApplyResult{}, fmt.Errorf("install staging session %q not found or expired", sessionID)
		}
		defer func() {
			_ = s.sessions.RemoveSession(sessionID)
		}()

		ctx := s.runtime.context()

		worker, err := uassettool.NewWriteWorker(nil)
		if err != nil {
			return install.ApplyResult{}, err
		}
		defer func() {
			_ = worker.Close() // Worker.Close waits for shutdown and always returns nil.
		}()
		onProgress := func(p install.Progress) {
			s.runtime.emit("install:progress", p)
		}
		if err := install.EncryptStagedMods(ctx, session, items, worker, onProgress); err != nil {
			return install.ApplyResult{}, err
		}
		if err := stripStagedCompanionPaks(session, items, worker); err != nil {
			return install.ApplyResult{}, err
		}

		onProgress(install.Progress{Phase: "applying", Message: "Install mods..."})
		result, err := install.Apply(ctx, modRoot, session, items, s.game)
		if err != nil {
			return result, err
		}

		if err := s.recordInstalledModMetadata(result, source); err != nil {
			// Files are already in the library. Wails rejects the JS promise when
			// the error is non-nil and discards ApplyResult, so the preview would
			// show a failed install and skip the library refresh.
			return result, nil
		}
		return result, nil
	})
}

// CancelInstall cleans up staging data when the user cancels the installation preview.
func (s *InstallService) CancelInstall(sessionID string) error {
	s.takeSource(sessionID)
	return s.sessions.RemoveSession(sessionID)
}

func (s *InstallService) recordInstalledModMetadata(result install.ApplyResult, source *nexusSource) error {
	err := s.meta.update(func(doc *metadata.Document) error {
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

// Removes and returns the Nexus source recorded for sessionID, or nil for a
// local-file session.
func (s *InstallService) takeSource(sessionID string) *nexusSource {
	s.sourcesMu.Lock()
	defer s.sourcesMu.Unlock()
	source, ok := s.sources[sessionID]
	if !ok {
		return nil
	}
	delete(s.sources, sessionID)
	return &source
}

// Removes every staging session when Cratebug closes.
func (s *InstallService) close() error {
	return s.sessions.CleanupAll()
}

func markStagedCompanionPaks(session *install.StagedSession, caller companionPakCaller) {
	root, err := filepath.Abs(session.Dir)
	if err != nil {
		return
	}
	for i := range session.Mods {
		mod := &session.Mods[i]
		pakAbs := filepath.Join(root, filepath.FromSlash(mod.RelativePrimaryPath))
		utocAbs := ""
		if mod.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(mod.Sidecars.UTOC))
		}
		needed, err := mutation.CompanionPakNeedsCleanup(pakAbs, utocAbs, caller)
		if err != nil {
			continue
		}
		mod.UnsupportedCompanionPak = needed
	}
}

func stripStagedCompanionPaks(session *install.StagedSession, items []install.ApplyItem, caller companionPakCaller) error {
	selected := make(map[string]struct{}, len(items))
	for _, item := range items {
		selected[item.ID] = struct{}{}
	}
	root, err := filepath.Abs(session.Dir)
	if err != nil {
		return fmt.Errorf("resolve staging directory: %w", err)
	}
	for _, mod := range session.Mods {
		if _, ok := selected[mod.ID]; !ok {
			continue
		}
		if mod.RelativePrimaryPath == "" {
			continue
		}
		pakAbs := filepath.Join(root, filepath.FromSlash(mod.RelativePrimaryPath))
		utocAbs := ""
		if mod.Sidecars.UTOC != "" {
			utocAbs = filepath.Join(root, filepath.FromSlash(mod.Sidecars.UTOC))
		}
		needed, err := mutation.CompanionPakNeedsCleanup(pakAbs, utocAbs, caller)
		if err != nil {
			// A file that is not a readable Unreal PAK is not the
			// chunknames problem. Leave it for Apply to copy as-is.
			continue
		}
		if !needed {
			continue
		}
		if err := mutation.RewriteCompanionPak(root, pakAbs, utocAbs, caller); err != nil {
			return fmt.Errorf("rewrite staged companion PAK %q: %w", mod.DisplayName, err)
		}
	}
	return nil
}
