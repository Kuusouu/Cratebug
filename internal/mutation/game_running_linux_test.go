//go:build linux

package mutation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxGameRunningCheckerDetectsDirectExecutable(t *testing.T) {
	// Arrange
	procDir := t.TempDir()
	pidDir := filepath.Join(procDir, "12345")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmdline := "Z:\\Steam\\steamapps\\common\\MarvelRivals\\MarvelGame\\Marvel\\Binaries\\Win64\\Marvel-Win64-Shipping.exe\x00-windowed\x00"
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("marvel-win64-sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checker := NewLinuxGameRunningCheckerForProc(procDir)

	// Act
	running, err := checker.IsGameRunning()

	// Assert
	if err != nil {
		t.Fatalf("IsGameRunning() unexpected error: %v", err)
	}
	if !running {
		t.Errorf("IsGameRunning() = false, want true")
	}
}

func TestLinuxGameRunningCheckerDetectsWineLoaderProcess(t *testing.T) {
	// Arrange
	procDir := t.TempDir()
	pidDir := filepath.Join(procDir, "67890")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmdline := "/usr/bin/wine64-preloader\x00Z:\\Steam\\steamapps\\common\\MarvelRivals\\MarvelGame\\Marvel\\Binaries\\Win64\\Marvel-Win64-Shipping.exe\x00-windowed\x00"
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("wine64-preloader\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checker := NewLinuxGameRunningCheckerForProc(procDir)

	// Act
	running, err := checker.IsGameRunning()

	// Assert
	if err != nil {
		t.Fatalf("IsGameRunning() unexpected error: %v", err)
	}
	if !running {
		t.Errorf("IsGameRunning() = false, want true")
	}
}

func TestLinuxGameRunningCheckerDetectsWineTruncatedComm(t *testing.T) {
	// Arrange
	procDir := t.TempDir()
	pidDir := filepath.Join(procDir, "99999")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("marvel-win64-sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checker := NewLinuxGameRunningCheckerForProc(procDir)

	// Act
	running, err := checker.IsGameRunning()

	// Assert
	if err != nil {
		t.Fatalf("IsGameRunning() unexpected error: %v", err)
	}
	if !running {
		t.Errorf("IsGameRunning() = false, want true")
	}
}

func TestLinuxGameRunningCheckerIgnoresUnrelatedProcesses(t *testing.T) {
	// Arrange
	procDir := t.TempDir()

	// PID 1: init system
	pid1 := filepath.Join(procDir, "1")
	if err := os.MkdirAll(pid1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid1, "cmdline"), []byte("/sbin/init\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid1, "comm"), []byte("systemd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// PID 2: grep process matching string
	pid2 := filepath.Join(procDir, "2")
	if err := os.MkdirAll(pid2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid2, "cmdline"), []byte("grep\x00-i\x00marvel-win64-shipping.exe\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid2, "comm"), []byte("grep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// PID 3: text editor viewing code
	pid3 := filepath.Join(procDir, "3")
	if err := os.MkdirAll(pid3, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid3, "cmdline"), []byte("nvim\x00internal/mutation/marvel-win64-shipping.exe\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid3, "comm"), []byte("nvim\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checker := NewLinuxGameRunningCheckerForProc(procDir)

	// Act
	running, err := checker.IsGameRunning()

	// Assert
	if err != nil {
		t.Fatalf("IsGameRunning() unexpected error: %v", err)
	}
	if running {
		t.Errorf("IsGameRunning() = true, want false for unrelated processes")
	}
}

func TestLinuxGameRunningCheckerFailsOnMissingProcDirectory(t *testing.T) {
	// Arrange
	procDir := filepath.Join(t.TempDir(), "nonexistent-proc")
	checker := NewLinuxGameRunningCheckerForProc(procDir)

	// Act
	running, err := checker.IsGameRunning()

	// Assert
	if err == nil {
		t.Fatal("IsGameRunning() expected error for missing directory, got nil")
	}
	if running {
		t.Errorf("IsGameRunning() = true, want false on error")
	}
}
