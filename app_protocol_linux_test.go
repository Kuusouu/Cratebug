//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppImageProtocolUsesPersistentExecutablePath(t *testing.T) {
	// Arrange
	appImage := filepath.Join(t.TempDir(), "Cratebug-x86_64.AppImage")
	if err := os.WriteFile(appImage, []byte("appimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", appImage)

	// Act
	protocolPath, err := protocolExecutablePath()

	// Assert
	if err != nil || protocolPath != appImage {
		t.Errorf("protocolExecutablePath() = %q, %v, want %q", protocolPath, err, appImage)
	}
}
