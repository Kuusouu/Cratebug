package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/urlscheme"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) onSecondInstanceLaunch(_ options.SecondInstanceData) {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()
	if ctx == nil {
		return
	}
	wailsRuntime.WindowUnminimise(ctx)
	wailsRuntime.WindowShow(ctx)
}

// Older builds saved a personal key and could displace another nxm:// handler.
// Remove the credential without decrypting it, and restore the previous handler
// only while Cratebug still owns the scheme. Keep existing mod metadata intact.
func cleanupLegacyNexus(store metadata.Store, registrar *urlscheme.Registrar, keyPath string) error {
	var keyErr error
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		keyErr = fmt.Errorf("remove legacy Nexus credential: %w", err)
	}

	doc, _ := store.Load()
	previous := doc.Settings.NexusProtocol
	var protocolErr error
	status, err := registrar.Status()
	if err != nil {
		protocolErr = fmt.Errorf("inspect legacy nxm handler: %w", err)
	} else if strings.EqualFold(filepath.Clean(status.OwnerPath), filepath.Clean(registrar.Exe)) {
		// A dev build or another installation with the same executable name
		// must not remove the installed application's handler.
		if err := registrar.Unregister(urlscheme.Snapshot{
			Command:     previous.Command,
			Icon:        previous.Icon,
			Description: previous.Description,
			DesktopFile: previous.DesktopFile,
		}); err != nil {
			protocolErr = fmt.Errorf("restore legacy nxm handler: %w", err)
		}
	}
	return errors.Join(keyErr, protocolErr)
}

func runUninstallCleanup() error {
	exe, err := protocolExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve executable for uninstall cleanup: %w", err)
	}
	path, err := metadata.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve metadata storage location: %w", err)
	}
	return cleanupLegacyNexus(metadata.NewStore(path), urlscheme.New(urlscheme.SchemeNXM, exe), filepath.Join(filepath.Dir(path), "nexus.key"))
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
