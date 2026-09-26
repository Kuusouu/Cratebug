//go:build linux

package mutation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// Directory permissions for trash metadata and files directories (owner-only access).
	trashDirMode os.FileMode = 0o700

	// File permissions for .trashinfo metadata files (owner read/write).
	trashInfoFileMode os.FileMode = 0o600

	// ISO-8601 deletion date format required by the FreeDesktop Trash specification.
	trashDateFormat = "2006-01-02T15:04:05"
)

var (
	gioLookPath = exec.LookPath
	gioCommand  = exec.Command
)

// recycleFiles moves paths to the desktop trash.
// It first attempts gio trash, then falls back to the FreeDesktop.org Trash specification.
func recycleFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("Recycle Bin operation has no paths")
	}

	for _, path := range paths {
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("verify path before trash: %w", err)
		}
	}

	if gioPath, err := gioLookPath("gio"); err == nil && gioPath != "" {
		cmd := gioCommand(gioPath, append([]string{"trash"}, paths...)...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return trashViaFreeDesktop(paths, "")
}

// trashViaFreeDesktop implements the FreeDesktop.org Trash specification under $XDG_DATA_HOME/Trash
// or a custom trash base directory for testing.
func trashViaFreeDesktop(paths []string, customTrashDir string) error {
	trashDir := customTrashDir
	if trashDir == "" {
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve user home for trash directory: %w", err)
			}
			dataHome = filepath.Join(home, ".local", "share")
		}
		trashDir = filepath.Join(dataHome, "Trash")
	}

	filesDir := filepath.Join(trashDir, "files")
	infoDir := filepath.Join(trashDir, "info")

	if err := os.MkdirAll(filesDir, trashDirMode); err != nil {
		return fmt.Errorf("create trash files directory: %w", err)
	}
	if err := os.MkdirAll(infoDir, trashDirMode); err != nil {
		return fmt.Errorf("create trash info directory: %w", err)
	}

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve absolute path for %q: %w", path, err)
		}

		baseName := filepath.Base(absPath)
		candidateName := baseName
		counter := 1

		for {
			targetFilePath := filepath.Join(filesDir, candidateName)
			targetInfoPath := filepath.Join(infoDir, candidateName+".trashinfo")
			_, fileErr := os.Lstat(targetFilePath)
			_, infoErr := os.Lstat(targetInfoPath)
			if os.IsNotExist(fileErr) && os.IsNotExist(infoErr) {
				break
			}
			candidateName = fmt.Sprintf("%s.%d", baseName, counter)
			counter++
		}

		infoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			absPath,
			time.Now().Format(trashDateFormat),
		)

		destInfoPath := filepath.Join(infoDir, candidateName+".trashinfo")
		if err := os.WriteFile(destInfoPath, []byte(infoContent), trashInfoFileMode); err != nil {
			return fmt.Errorf("write trash info for %q: %w", path, err)
		}

		destFilePath := filepath.Join(filesDir, candidateName)
		if err := os.Rename(absPath, destFilePath); err != nil {
			if errors.Is(err, syscall.EXDEV) {
				if moveErr := moveCrossDevice(absPath, destFilePath); moveErr != nil {
					_ = os.Remove(destInfoPath)
					return fmt.Errorf("move %q across devices to trash files: %w", path, moveErr)
				}
			} else {
				_ = os.Remove(destInfoPath)
				return fmt.Errorf("move %q to trash files: %w", path, err)
			}
		}
	}

	return nil
}

func moveCrossDevice(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := copyDir(src, dst); err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
		return os.RemoveAll(src)
	}

	if err := copyFile(src, dst, info.Mode()); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyFile(s, d, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
