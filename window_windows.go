//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	spiGetWorkArea = 0x0030
	defaultDPI     = 96
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	systemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	getDpiForSystem       = user32.NewProc("GetDpiForSystem")
)

// Reports the primary monitor work area in the same logical (96-DPI) pixels
// Wails Width and Height use. SPI_GETWORKAREA and GetDpiForSystem virtualize
// together when this code runs before wails.Run has opted into DPI awareness,
// and return physical pixels plus real DPI when the application manifest has
// already enabled per-monitor awareness.
func primaryWorkArea() (int, int) {
	var area windows.Rect
	ok, _, _ := systemParametersInfoW.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&area)),
		0,
	)
	if ok == 0 {
		return 0, 0
	}

	width := int(area.Right - area.Left)
	height := int(area.Bottom - area.Top)
	if width <= 0 || height <= 0 {
		return 0, 0
	}

	dpi := defaultDPI
	if err := getDpiForSystem.Find(); err == nil {
		if raw, _, _ := getDpiForSystem.Call(); raw != 0 {
			dpi = int(raw)
		}
	}

	return width * defaultDPI / dpi, height * defaultDPI / dpi
}
