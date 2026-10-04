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
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Default filename for the backup save dialog, stamped so repeated backups
// do not overwrite each other without asking.
const backupFilenameTimestampLayout = "2006-01-02-150405"

// Guards the in-flight backup cancellation. Stored on App so the
// CancelBackup binding can reach it; see the encrypt/companion pattern.
var (
	backupMu     sync.Mutex
	backupCancel context.CancelFunc
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
