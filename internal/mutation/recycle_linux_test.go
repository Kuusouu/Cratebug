//go:build linux

package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashViaFreeDesktopMovesFileAndCreatesTrashInfo(t *testing.T) {
	// Arrange
	trashDir := t.TempDir()
	sourceDir := t.TempDir()
	filePath := filepath.Join(sourceDir, "testmod.pak")
	content := []byte("mod data content")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := trashViaFreeDesktop([]string{filePath}, trashDir)

	// Assert
	if err != nil {
		t.Fatalf("trashViaFreeDesktop unexpected error: %v", err)
	}
	if _, err := os.Lstat(filePath); !os.IsNotExist(err) {
		t.Errorf("original file still exists at %q", filePath)
	}

	trashedFile := filepath.Join(trashDir, "files", "testmod.pak")
	trashedData, err := os.ReadFile(trashedFile)
	if err != nil {
		t.Fatalf("read trashed file: %v", err)
	}
	if string(trashedData) != string(content) {
		t.Errorf("trashed content = %q, want %q", string(trashedData), string(content))
	}

	infoPath := filepath.Join(trashDir, "info", "testmod.pak.trashinfo")
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read trash info file: %v", err)
	}
	infoStr := string(infoData)
	if !strings.Contains(infoStr, "[Trash Info]") {
		t.Errorf("trash info missing [Trash Info] header: %s", infoStr)
	}
	if !strings.Contains(infoStr, "Path="+filePath) {
		t.Errorf("trash info missing Path=%s: %s", filePath, infoStr)
	}
	if !strings.Contains(infoStr, "DeletionDate=") {
		t.Errorf("trash info missing DeletionDate: %s", infoStr)
	}
}

func TestTrashViaFreeDesktopResolvesNameCollisions(t *testing.T) {
	// Arrange
	trashDir := t.TempDir()
	filesDir := filepath.Join(trashDir, "files")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(filesDir, "collision.pak")
	if err := os.WriteFile(existingPath, []byte("existing in trash"), 0o600); err != nil {
		t.Fatal(err)
	}

	sourceDir := t.TempDir()
	filePath := filepath.Join(sourceDir, "collision.pak")
	newContent := []byte("new file trashed")
	if err := os.WriteFile(filePath, newContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := trashViaFreeDesktop([]string{filePath}, trashDir)

	// Assert
	if err != nil {
		t.Fatalf("trashViaFreeDesktop unexpected error: %v", err)
	}
	collidedFile := filepath.Join(trashDir, "files", "collision.pak.1")
	collidedData, err := os.ReadFile(collidedFile)
	if err != nil {
		t.Fatalf("read collided file %q: %v", collidedFile, err)
	}
	if string(collidedData) != string(newContent) {
		t.Errorf("collided content = %q, want %q", string(collidedData), string(newContent))
	}

	collidedInfo := filepath.Join(trashDir, "info", "collision.pak.1.trashinfo")
	if _, err := os.Lstat(collidedInfo); err != nil {
		t.Errorf("missing collision info file: %v", err)
	}
}

func TestRecycleFilesFailsForNonexistentPath(t *testing.T) {
	// Arrange
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.pak")

	// Act
	err := recycleFiles([]string{nonexistent})

	// Assert
	if err == nil {
		t.Fatal("recycleFiles expected error for nonexistent path, got nil")
	}
}

func TestRecycleFilesFailsForEmptyPaths(t *testing.T) {
	// Arrange
	paths := []string{}

	// Act
	err := recycleFiles(paths)

	// Assert
	if err == nil {
		t.Fatal("recycleFiles expected error for empty paths, got nil")
	}
}

func TestMoveCrossDeviceFile(t *testing.T) {
	// Arrange
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "mod.pak")
	dstFile := filepath.Join(dstDir, "mod.pak")
	data := []byte("mod content payload")
	if err := os.WriteFile(srcFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := moveCrossDevice(srcFile, dstFile)

	// Assert
	if err != nil {
		t.Fatalf("moveCrossDevice unexpected error: %v", err)
	}
	if _, err := os.Lstat(srcFile); !os.IsNotExist(err) {
		t.Errorf("source file %q still exists after moveCrossDevice", srcFile)
	}
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("destination content = %q, want %q", string(got), string(data))
	}
}

func TestMoveCrossDeviceDirectory(t *testing.T) {
	// Arrange
	srcDir := filepath.Join(t.TempDir(), "source_mod")
	dstDir := filepath.Join(t.TempDir(), "dest_mod")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "sub", "mod.pak")
	data := []byte("nested mod data")
	if err := os.WriteFile(srcFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := moveCrossDevice(srcDir, dstDir)

	// Assert
	if err != nil {
		t.Fatalf("moveCrossDevice unexpected error: %v", err)
	}
	if _, err := os.Lstat(srcDir); !os.IsNotExist(err) {
		t.Errorf("source directory %q still exists after moveCrossDevice", srcDir)
	}
	got, err := os.ReadFile(filepath.Join(dstDir, "sub", "mod.pak"))
	if err != nil {
		t.Fatalf("read nested destination file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("destination content = %q, want %q", string(got), string(data))
	}
}
