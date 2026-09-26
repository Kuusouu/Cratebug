//go:build linux

package urlscheme

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserHiveWriteCreatesDesktopFileAndUpdatesMimeappsList(t *testing.T) {
	// Arrange
	appDir, configDir := setupLinuxHiveTest(t)

	snapshot := Snapshot{
		Command: `"/opt/cratebug/cratebug" "%1"`,
	}

	// Act
	err := userHive{}.write("nxm", snapshot)

	// Assert
	if err != nil {
		t.Fatalf("userHive.write unexpected error: %v", err)
	}

	desktopFile := filepath.Join(appDir, "cratebug-nxm.desktop")
	content, err := os.ReadFile(desktopFile)
	if err != nil {
		t.Fatalf("read desktop file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, `Exec="/opt/cratebug/cratebug" %u`) {
		t.Errorf("desktop file missing expected Exec line: %s", contentStr)
	}
	if !strings.Contains(contentStr, "MimeType=x-scheme-handler/nxm;") {
		t.Errorf("desktop file missing expected MimeType line: %s", contentStr)
	}

	mimeappsPath := filepath.Join(configDir, "mimeapps.list")
	mimeContent, err := os.ReadFile(mimeappsPath)
	if err != nil {
		t.Fatalf("read mimeapps.list: %v", err)
	}
	mimeStr := string(mimeContent)
	if !strings.Contains(mimeStr, "x-scheme-handler/nxm=cratebug-nxm.desktop") {
		t.Errorf("mimeapps.list missing association: %s", mimeStr)
	}
}

func TestUserHiveReadParsesDesktopFileFromMimeapps(t *testing.T) {
	// Arrange
	appDir, configDir := setupLinuxHiveTest(t)

	desktopContent := `[Desktop Entry]
Type=Application
Name=Vortex
Exec=/usr/bin/vortex %u
MimeType=x-scheme-handler/nxm;
`
	if err := os.WriteFile(filepath.Join(appDir, "vortex.desktop"), []byte(desktopContent), 0o644); err != nil {
		t.Fatal(err)
	}

	mimeContent := `[Default Applications]
x-scheme-handler/nxm=vortex.desktop
`
	if err := os.WriteFile(filepath.Join(configDir, "mimeapps.list"), []byte(mimeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	snapshot, exists, err := userHive{}.read("nxm")

	// Assert
	if err != nil {
		t.Fatalf("userHive.read unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("userHive.read reported not exists, want exists=true")
	}
	if want := `/usr/bin/vortex`; commandExecutablePath(snapshot.Command) != want {
		t.Errorf("executable path = %q, want %q", commandExecutablePath(snapshot.Command), want)
	}
	if snapshot.Command != "/usr/bin/vortex %u" || snapshot.DesktopFile != "vortex.desktop" {
		t.Errorf("snapshot = %+v, want the exact Exec command and desktop file", snapshot)
	}
}

func TestUserHiveDeleteRemovesDesktopFileAndMimeAssociation(t *testing.T) {
	// Arrange
	appDir, configDir := setupLinuxHiveTest(t)

	if err := (userHive{}).write("nxm", Snapshot{Command: `"/usr/bin/cratebug" "%1"`}); err != nil {
		t.Fatal(err)
	}

	// Act
	err := userHive{}.delete("nxm")

	// Assert
	if err != nil {
		t.Fatalf("userHive.delete unexpected error: %v", err)
	}

	desktopFile := filepath.Join(appDir, "cratebug-nxm.desktop")
	if _, err := os.Lstat(desktopFile); !os.IsNotExist(err) {
		t.Errorf("desktop file still exists at %q", desktopFile)
	}

	mimeappsPath := filepath.Join(configDir, "mimeapps.list")
	if found := readMimeDefault(mimeappsPath, "x-scheme-handler/nxm"); found != "" {
		t.Errorf("mime default still contains %q, want empty", found)
	}
}

func TestRegistrarLifecycleOnLinux(t *testing.T) {
	// Arrange
	appDir, _ := setupLinuxHiveTest(t)

	selfExe := filepath.Join(appDir, "cratebug")
	if err := os.WriteFile(selfExe, []byte("executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	registrar := New("nxm", selfExe)

	// Act 1: Initial status
	initialStatus, err := registrar.Status()

	// Assert 1:
	if err != nil {
		t.Fatalf("Status unexpected error: %v", err)
	}
	if initialStatus.Ownership != OwnershipNone {
		t.Errorf("initial Ownership = %q, want %q", initialStatus.Ownership, OwnershipNone)
	}

	// Act 2: Register
	displaced, err := registrar.Register(false)

	// Assert 2:
	if err != nil {
		t.Fatalf("Register unexpected error: %v", err)
	}
	if !displaced.Empty() {
		t.Errorf("displaced snapshot = %+v, want empty", displaced)
	}

	statusAfterRegister, err := registrar.Status()
	if err != nil {
		t.Fatalf("Status after register error: %v", err)
	}
	if statusAfterRegister.Ownership != OwnershipSelf {
		t.Errorf("Ownership = %q, want %q", statusAfterRegister.Ownership, OwnershipSelf)
	}

	// Act 3: Unregister
	err = registrar.Unregister(Snapshot{})

	// Assert 3:
	if err != nil {
		t.Fatalf("Unregister unexpected error: %v", err)
	}
	statusAfterUnregister, err := registrar.Status()
	if err != nil {
		t.Fatalf("Status after unregister error: %v", err)
	}
	if statusAfterUnregister.Ownership != OwnershipNone {
		t.Errorf("Ownership after unregister = %q, want %q", statusAfterUnregister.Ownership, OwnershipNone)
	}
}

func TestRegistrarRegistersWithoutExistingConfigDirectory(t *testing.T) {
	appDir, configDir := setupLinuxHiveTest(t)
	configDirOverride = filepath.Join(configDir, "not-yet-created")
	selfExe := filepath.Join(appDir, "cratebug")
	if err := os.WriteFile(selfExe, []byte("executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := New("nxm", selfExe).Register(false); err != nil {
		t.Fatalf("Register() with no config directory: %v", err)
	}
}

func TestRegistrarRestoresForeignDesktopAssociation(t *testing.T) {
	// Arrange
	appDir, configDir := setupLinuxHiveTest(t)
	foreignContent := "[Desktop Entry]\nType=Application\nExec=flatpak run --branch=stable org.vortex.App %u\n"
	foreignPath := filepath.Join(appDir, "org.vortex.App.desktop")
	if err := os.WriteFile(foreignPath, []byte(foreignContent), 0o644); err != nil {
		t.Fatal(err)
	}
	mimeappsPath := filepath.Join(configDir, "mimeapps.list")
	if err := os.WriteFile(mimeappsPath, []byte("[Default Applications]\nx-scheme-handler/nxm=org.vortex.App.desktop;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selfExe := filepath.Join(appDir, "Cratebug-x86_64.AppImage")
	if err := os.WriteFile(selfExe, []byte("appimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	registrar := New("nxm", selfExe)

	// Act
	displaced, err := registrar.Register(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := registrar.Unregister(displaced); err != nil {
		t.Fatal(err)
	}

	// Assert
	if displaced.DesktopFile != "org.vortex.App.desktop" || displaced.Command != "flatpak run --branch=stable org.vortex.App %u" {
		t.Errorf("displaced snapshot = %+v, want exact foreign association", displaced)
	}
	content, err := os.ReadFile(foreignPath)
	if err != nil || string(content) != foreignContent {
		t.Errorf("foreign desktop file changed: %v", err)
	}
	if got := readMimeDefault(mimeappsPath, "x-scheme-handler/nxm"); got != displaced.DesktopFile {
		t.Errorf("restored default = %q, want %q", got, displaced.DesktopFile)
	}
}

func TestFindDesktopFileUsesXDGDataDirs(t *testing.T) {
	setupLinuxHiveTest(t)
	dataDir := t.TempDir()
	applicationsDir := filepath.Join(dataDir, "applications")
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(applicationsDir, "foreign.desktop")
	if err := os.WriteFile(want, []byte("[Desktop Entry]\nExec=foreign %u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_DIRS", dataDir)

	got, err := findDesktopFile("foreign.desktop")
	if err != nil || got != want {
		t.Fatalf("findDesktopFile() = %q, %v; want %q", got, err, want)
	}
}

func setupLinuxHiveTest(t *testing.T) (string, string) {
	t.Helper()
	appDir := t.TempDir()
	configDir := t.TempDir()
	previousCommand := xdgMimeCommand
	applicationsDirOverride = appDir
	configDirOverride = configDir
	xdgMimeCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("cratebug-test-command-unavailable")
	}
	t.Cleanup(func() {
		applicationsDirOverride = ""
		configDirOverride = ""
		xdgMimeCommand = previousCommand
	})
	return appDir, configDir
}
