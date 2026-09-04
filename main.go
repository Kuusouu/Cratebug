package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// AppVersion is the running build's CalVer release tag (e.g. "2026.08.27" or
// "2026.08.27-rc1"). The release workflow overrides it at build time via
// -ldflags "-X main.AppVersion=...". Left at "dev" for local and CI builds,
// which never claim to be a real release for update-check purposes.
var AppVersion = "dev"

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	window := defaultWindowSize()
	err = wails.Run(&options.App{
		Title:     "Cratebug",
		Width:     window.width,
		Height:    window.height,
		MinWidth:  window.minWidth,
		MinHeight: window.minHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 12, G: 16, B: 24, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			// Prevents the WebView from navigating to a dropped file if our own
			// frontend handler ever fails to intercept the drop.
			DisableWebViewDrop: true,
		},
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
	})
	if err != nil {
		log.Fatal(err)
	}
}
