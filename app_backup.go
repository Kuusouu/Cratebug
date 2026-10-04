package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/backup"
	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/mutation"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Default filename for the backup save dialog, stamped so repeated backups
// do not overwrite each other without asking.
const backupFilenameTimestampLayout = "2006-01-02-150405"

// Guards in-flight backup and restore cancellation. Stored on App so the
// CancelBackup and CancelRestore bindings can reach them; see the
// encrypt/companion pattern.
var (
	backupMu      sync.Mutex
	backupCancel  context.CancelFunc
	restoreMu     sync.Mutex
	restoreCancel context.CancelFunc
)

// BackupType anchors backup.Result so Wails emits its TypeScript model.
func (a *App) BackupType() backup.Result {
	return backup.Result{}
}

// Backs up the mod-root tree and Cratebug metadata to a user-chosen zip
// file. Read-only against the library, so no game-running check and no
// watcher suppression. Emits backup:progress events. A cancelled save dialog
// returns a cancelled result, not an error, so the frontend stays silent.
func (a *App) BackupLibrary(modRoot string) (backup.Result, error) {
	if a.ctx == nil {
		return backup.Result{}, fmt.Errorf("application runtime context is not available")
	}

	destPath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "Save Library Backup",
		DefaultFilename: "CratebugBackup-" + time.Now().Format(backupFilenameTimestampLayout) + ".zip",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Backup Archives (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil {
		return backup.Result{}, fmt.Errorf("open save dialog: %w", err)
	}
	if strings.TrimSpace(destPath) == "" {
		return backup.Result{Cancelled: true}, nil
	}
	if !strings.EqualFold(filepath.Ext(destPath), ".zip") {
		destPath += ".zip"
	}

	metadataPath, err := metadata.DefaultPath()
	if err != nil {
		return backup.Result{}, fmt.Errorf("resolve metadata location: %w", err)
	}

	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, stop := context.WithCancel(base)
	backupMu.Lock()
	backupCancel = stop
	backupMu.Unlock()
	defer func() {
		backupMu.Lock()
		backupCancel = nil
		backupMu.Unlock()
	}()

	result, err := backup.Create(ctx, modRoot, metadataPath, destPath, func(progress backup.Progress) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "backup:progress", progress)
		}
	})
	if err != nil {
		return backup.Result{}, err
	}
	return result, nil
}

// Stops an in-progress BackupLibrary; the partial destination is removed.
func (a *App) CancelBackup() {
	backupMu.Lock()
	defer backupMu.Unlock()
	if backupCancel != nil {
		backupCancel()
		backupCancel = nil
	}
}

// RestorePreviewType anchors backup.Preview so Wails emits its TypeScript model.
func (a *App) RestorePreviewType() backup.Preview {
	return backup.Preview{}
}

// RestoreResultType anchors backup.RestoreResult so Wails emits its TypeScript model.
func (a *App) RestoreResultType() backup.RestoreResult {
	return backup.RestoreResult{}
}

// Stages a user-chosen backup zip for a later RestoreApply. Read-only:
// nothing is validated against the game-running lock at this point. A
// dismissed open dialog returns a cancelled preview, not an error, so the
// frontend stays silent.
func (a *App) RestorePreview() (backup.Preview, error) {
	if a.ctx == nil {
		return backup.Preview{}, fmt.Errorf("application runtime context is not available")
	}

	zipPath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Library Backup",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Backup Archives (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil {
		return backup.Preview{}, fmt.Errorf("open file dialog: %w", err)
	}
	if strings.TrimSpace(zipPath) == "" {
		return backup.Preview{Cancelled: true}, nil
	}

	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return a.restoreSessions.Preview(base, zipPath)
}

// Replaces the mod-root tree with a staged preview's contents and restores
// its metadata. Blocked while Marvel Rivals runs. Emits restore:progress
// events as entries move into place.
func (a *App) RestoreApply(modRoot, token string) (backup.RestoreResult, error) {
	return suppressWatcherResult(a, func() (backup.RestoreResult, error) {
		if a.gameRunningChecker != nil {
			running, err := a.gameRunningChecker.IsGameRunning()
			if err != nil {
				return backup.RestoreResult{}, fmt.Errorf("check whether Marvel Rivals is running: %w", err)
			}
			if running {
				return backup.RestoreResult{}, mutation.ErrGameRunning
			}
		}

		metadataPath, err := metadata.DefaultPath()
		if err != nil {
			return backup.RestoreResult{}, fmt.Errorf("resolve metadata location: %w", err)
		}

		base := a.ctx
		if base == nil {
			base = context.Background()
		}
		ctx, stop := context.WithCancel(base)
		restoreMu.Lock()
		if restoreCancel != nil {
			restoreMu.Unlock()
			stop()
			return backup.RestoreResult{}, fmt.Errorf("a restore is already active")
		}
		restoreCancel = stop
		restoreMu.Unlock()
		defer func() {
			stop()
			restoreMu.Lock()
			restoreCancel = nil
			restoreMu.Unlock()
		}()

		return a.restoreSessions.Apply(ctx, modRoot, metadataPath, token, func(progress backup.Progress) {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "restore:progress", progress)
			}
		})
	})
}

// Drops a staged preview without touching the library.
func (a *App) DiscardRestorePreview(token string) {
	a.restoreSessions.DiscardSession(token)
}

// Keeps retry controls tied to a complete, idle staged backup.
func (a *App) CanRetryRestore(token string) bool {
	return a.restoreSessions.CanRetry(token)
}

// Stops an in-progress RestoreApply; the previous library is put back.
func (a *App) CancelRestore() {
	restoreMu.Lock()
	defer restoreMu.Unlock()
	if restoreCancel != nil {
		restoreCancel()
	}
}
