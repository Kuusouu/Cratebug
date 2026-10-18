package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Wails exposes every bound method to the frontend with no lower trust
// boundary, so ApplyUpdate can't assume installerPath is always exactly
// what DownloadUpdate produced. These only exercise the rejection paths:
// the acceptance path calls update.ApplyUpdate, which spawns a real
// detached process and isn't something a unit test should trigger.
func TestApplyUpdate_RejectsPathOutsideExpectedDirectory(t *testing.T) {
	// Arrange
	updates := testApp(t, false).updates
	outsidePath := filepath.Join(t.TempDir(), "installer.exe")

	// Act
	err := updates.ApplyUpdate(outsidePath)

	// Assert
	if err == nil {
		t.Fatal("ApplyUpdate succeeded for a path outside the expected download directory, want an error")
	}
}

func TestApplyUpdate_RejectsNonExeExtension(t *testing.T) {
	// Arrange
	updates := testApp(t, false).updates
	insidePath := filepath.Join(os.TempDir(), updateDownloadDirName, "installer.bat")

	// Act
	err := updates.ApplyUpdate(insidePath)

	// Assert
	if err == nil {
		t.Fatal("ApplyUpdate succeeded for a non-.exe path, want an error")
	}
}

func TestApplyUpdate_RejectsPathTraversal(t *testing.T) {
	// Arrange
	updates := testApp(t, false).updates
	traversalPath := filepath.Join(os.TempDir(), updateDownloadDirName, "..", "..", "System32", "evil.exe")

	// Act
	err := updates.ApplyUpdate(traversalPath)

	// Assert
	if err == nil {
		t.Fatal("ApplyUpdate succeeded for a path traversal attempt, want an error")
	}
}

// A path that passes the shape checks but points at nothing must fail before
// the helper is launched and the app quits, or a vanished installer (e.g.
// antivirus quarantine) would close Cratebug with no relaunch and no error.
func TestApplyUpdate_RejectsMissingInstallerFile(t *testing.T) {
	// Arrange
	updates := testApp(t, false).updates
	missingPath := filepath.Join(os.TempDir(), updateDownloadDirName, "missing-installer.exe")

	// Act
	err := updates.ApplyUpdate(missingPath)

	// Assert
	if err == nil {
		t.Fatal("ApplyUpdate succeeded for an installer file that does not exist, want an error")
	}
}
