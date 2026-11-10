package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/gamedetect"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	"github.com/Kuusouu/Cratebug/internal/patchupdate"
	"github.com/Kuusouu/Cratebug/internal/reveal"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Carries mods across a game patch that moved their asset layout, bound to
// the frontend.
type PatchUpdateService struct {
	*core

	// Overridable in tests so no run needs the network or the user's config.
	depot     func() (patchupdate.Depot, error)
	originals func(build int) (string, error)

	mu     sync.Mutex
	cancel context.CancelFunc
	// Closed when the current check or update has stopped. begin waits on it
	// so a dialog reopened while a cancelled run winds down starts cleanly
	// instead of reporting that another update is running.
	done chan struct{}
}

func newPatchUpdateService(c *core) *PatchUpdateService {
	return &PatchUpdateService{core: c, depot: defaultPatchDepot, originals: defaultPatchOriginals}
}

func defaultPatchDepot() (patchupdate.Depot, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return patchupdate.Depot{}, fmt.Errorf("resolve mappings cache: %w", err)
	}
	return patchupdate.Depot{
		ListURL:  patchupdate.DefaultDepotListURL,
		CacheDir: filepath.Join(configDir, "Cratebug", "usmap"),
	}, nil
}

func defaultPatchOriginals(build int) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve originals folder: %w", err)
	}
	return filepath.Join(configDir, "Cratebug", "patch-update-originals", strconv.Itoa(build)), nil
}

// CheckModsForPatch classifies the complete IoStore mods named by targets, or
// the whole library when targets is empty, against the current game layout
// without changing any file. The result is a plan the user can review and
// select from.
func (s *PatchUpdateService) CheckModsForPatch(modRoot string, targets []mutation.PatchUpdateTarget) (mutation.PatchUpdatePlan, error) {
	ctx := s.begin()
	defer s.end()

	paks, _, layouts, err := s.loadPatchContext(ctx, modRoot)
	if err != nil {
		return mutation.PatchUpdatePlan{}, err
	}
	request := mutation.PatchUpdateRequest{
		ModRoot:  modRoot,
		Layouts:  layouts,
		GamePaks: paks,
		Targets:  targets,
	}
	return mutation.CheckModsForPatch(ctx, request, func(ctx context.Context, scratch string) (mutation.ArchiveCaller, func(), error) {
		worker, err := uassettool.NewPatchCheckWorker(ctx, scratch)
		if err != nil {
			return nil, nil, err
		}
		return worker, func() { _ = worker.Close() }, nil
	})
}

// UpdateModsForPatch rewrites only the selected mods in place. Each rewrite
// is verified before the live files change, and originals are kept first.
func (s *PatchUpdateService) UpdateModsForPatch(modRoot string, targets []mutation.PatchUpdateTarget) (mutation.PatchUpdateResult, error) {
	if len(targets) == 0 {
		return mutation.PatchUpdateResult{}, fmt.Errorf("select at least one mod to update")
	}
	return suppressWatcherResult(s.core, func() (mutation.PatchUpdateResult, error) {
		if err := s.requireGameNotRunning(); err != nil {
			return mutation.PatchUpdateResult{}, err
		}
		ctx := s.begin()
		defer s.end()

		paks, build, layouts, err := s.loadPatchContext(ctx, modRoot)
		if err != nil {
			return mutation.PatchUpdateResult{}, err
		}
		originals, err := s.originals(build)
		if err != nil {
			return mutation.PatchUpdateResult{}, err
		}
		request := mutation.PatchUpdateRequest{
			ModRoot:      modRoot,
			Layouts:      layouts,
			GamePaks:     paks,
			OriginalsDir: originals,
			Targets:      targets,
		}
		return mutation.UpdateModsForPatch(request, patchUpdateLaunch, s.emitProgress, ctx.Done())
	})
}

// Cancels a check or lets an update stop after the mods already in progress.
func (s *PatchUpdateService) CancelPatchUpdate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Opens the folder that keeps the originals of mods the updater rewrote.
func (s *PatchUpdateService) OpenPatchOriginals(build int) error {
	folder, err := s.originals(build)
	if err != nil {
		return err
	}
	if _, err := os.Stat(folder); err != nil {
		return fmt.Errorf("no originals are kept for build %d yet", build)
	}
	return reveal.OpenDirectory(folder)
}

func (s *PatchUpdateService) begin() context.Context {
	s.mu.Lock()
	for s.done != nil {
		done := s.done
		s.mu.Unlock()
		<-done
		s.mu.Lock()
	}
	ctx, cancel := context.WithCancel(s.runtime.context())
	s.cancel = cancel
	s.done = make(chan struct{})
	s.mu.Unlock()
	return ctx
}

func (s *PatchUpdateService) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}

func (s *PatchUpdateService) emitProgress(progress mutation.PatchUpdateProgress) {
	s.runtime.emit("patchupdate:progress", progress)
}

func patchUpdateLaunch() (mutation.ArchiveCaller, func(), error) {
	worker, err := uassettool.NewWriteWorker(nil)
	if err != nil {
		return nil, nil, err
	}
	return worker, func() { _ = worker.Close() }, nil
}

// Loads the game build and mappings once for a check or an update. The game
// is found above the library when possible, then through the store selected
// in Settings, so a backup or staging library outside the install still works.
func (s *PatchUpdateService) loadPatchContext(ctx context.Context, modRoot string) (string, int, *patchupdate.Layouts, error) {
	paks, err := s.resolveGamePaks(modRoot)
	if err != nil {
		return "", 0, nil, err
	}
	build, err := patchupdate.GameBuild(paks)
	if err != nil {
		return "", 0, nil, fmt.Errorf("read the game build from %s: %w", paks, err)
	}
	depot, err := s.depot()
	if err != nil {
		return "", 0, nil, err
	}
	layouts, err := patchupdate.LoadLayouts(ctx, depot, build, patchupdate.DefaultPreviousLayouts)
	if errors.Is(err, patchupdate.ErrMappingsNotPublished) {
		return "", 0, nil, fmt.Errorf("mappings for game build %d are not published yet", build)
	}
	if err != nil {
		return "", 0, nil, fmt.Errorf("load mappings for build %d: %w", build, err)
	}
	return paks, build, layouts, nil
}

func (s *PatchUpdateService) resolveGamePaks(modRoot string) (string, error) {
	if paks, err := patchupdate.FindGamePaks(modRoot); err == nil {
		return paks, nil
	}

	provider := s.meta.read().Settings.LibraryProvider
	if provider == "" {
		provider = gamedetect.ProviderSteam
	}
	detector := s.detector
	if detector == nil {
		detector = gamedetect.NewDefaultRegistry()
	}
	detection, err := detector.Detect(provider)
	if err != nil {
		return "", fmt.Errorf("find the Marvel Rivals installation: %w", err)
	}
	if detection.State == gamedetect.StateNotFound || detection.PaksPath == "" {
		return "", fmt.Errorf(
			"Marvel Rivals was not found in your %s library. Put the mod library inside the game's Paks folder, or choose the right store in Settings",
			storeLabel(provider),
		)
	}
	return detection.PaksPath, nil
}

func storeLabel(provider string) string {
	if provider == gamedetect.ProviderEpic {
		return "Epic Games"
	}
	return "Steam"
}
