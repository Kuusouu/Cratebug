package sigbypass

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func sha256Of(t *testing.T, content string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// Creates the Win64 directory shape that providers verify, with the
// shipping executable in place.
func writeGameInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	paksPath := filepath.Join(root, "MarvelGame", "Marvel", "Content", "Paks")
	win64 := filepath.Join(root, "MarvelGame", "Marvel", "Binaries", "Win64")
	if err := os.MkdirAll(paksPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(win64, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(win64, GameExecutableName), []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	return win64
}

// Makes a synthetic payload with per-test contents and the matching
// digests, standing in for the fetched release payload.
func writePayload(t *testing.T, loaderContent, pluginContent string) Payload {
	t.Helper()
	dir := t.TempDir()
	loader := LoaderFileName
	plugin := filepath.Join(PluginDirName, PluginFileName)
	if err := os.MkdirAll(filepath.Join(dir, PluginDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, loader), []byte(loaderContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plugin), []byte(pluginContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return Payload{
		Dir: dir,
		Files: []PayloadFile{
			{RelativePath: loader, SHA256: sha256Of(t, loaderContent)},
			{RelativePath: plugin, SHA256: sha256Of(t, pluginContent)},
		},
	}
}

func TestGameDirForPaksDerivesWin64FromPaks(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	paksPath := filepath.Join(filepath.Dir(win64), "..", "Content", "Paks")

	// Act
	got, err := GameDirForPaks(paksPath)

	// Assert
	if err != nil {
		t.Fatalf("GameDirForPaks() = %v", err)
	}
	if got != win64 {
		t.Errorf("GameDirForPaks() = %q, want %q", got, win64)
	}
}

func TestGameDirForPaksRejectsInstallWithoutExecutable(t *testing.T) {
	// Arrange
	root := t.TempDir()
	paksPath := filepath.Join(root, "MarvelGame", "Marvel", "Content", "Paks")
	if err := os.MkdirAll(paksPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := GameDirForPaks(paksPath)

	// Assert
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("GameDirForPaks() = %v, want ErrGameNotFound", err)
	}
}

func TestGameDirForPaksRejectsEmptyPath(t *testing.T) {
	// Act
	_, err := GameDirForPaks("")

	// Assert
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("GameDirForPaks() = %v, want ErrGameNotFound", err)
	}
}

func TestInspectReportsNotInstalledOnAnEmptyGameDir(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	files := writePayload(t, "loader-bytes", "plugin-bytes").Files

	// Act
	status, err := Inspect(win64, files)

	// Assert
	if err != nil {
		t.Fatalf("Inspect() = %v", err)
	}
	if status.State != StateNotInstalled {
		t.Errorf("State = %q, want %q", status.State, StateNotInstalled)
	}
	if status.GameDir != win64 {
		t.Errorf("GameDir = %q, want %q", status.GameDir, win64)
	}
	if len(status.Files) != len(files) {
		t.Fatalf("len(Files) = %d, want %d", len(status.Files), len(files))
	}
	for _, file := range status.Files {
		if file.State != FileMissing {
			t.Errorf("%s state = %q, want %q", file.RelativePath, file.State, FileMissing)
		}
	}
}

func TestInstallCopiesPayloadAndInspectReportsInstalled(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")

	// Act
	err := Install(win64, payload)

	// Assert
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	status, err := Inspect(win64, payload.Files)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateInstalled {
		t.Fatalf("State = %q, want %q", status.State, StateInstalled)
	}
	for _, file := range status.Files {
		if file.State != FileCurrent {
			t.Errorf("%s state = %q, want %q", file.RelativePath, file.State, FileCurrent)
		}
	}
	got, err := os.ReadFile(filepath.Join(win64, PluginDirName, PluginFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plugin-bytes" {
		t.Errorf("installed plugin = %q, want %q", got, "plugin-bytes")
	}
}

func TestInstallAndInspectTreatAPartialInstallAsRepairable(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	if err := Install(win64, payload); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(win64, PluginDirName, PluginFileName)); err != nil {
		t.Fatal(err)
	}

	// Act
	status, err := Inspect(win64, payload.Files)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StatePartial {
		t.Fatalf("State = %q, want %q", status.State, StatePartial)
	}

	// Act
	if err := Install(win64, payload); err != nil {
		t.Fatalf("repair Install() = %v", err)
	}

	// Assert
	status, err = Inspect(win64, payload.Files)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateInstalled {
		t.Fatalf("State after repair = %q, want %q", status.State, StateInstalled)
	}
}

func TestInstallRefusesToOverwriteADifferentFile(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	if err := os.WriteFile(filepath.Join(win64, LoaderFileName), []byte("other-loader"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := writePayload(t, "loader-bytes", "plugin-bytes")

	// Act
	err := Install(win64, payload)

	// Assert
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Install() = %v, want ErrConflict", err)
	}
	got, readErr := os.ReadFile(filepath.Join(win64, LoaderFileName))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "other-loader" {
		t.Error("install overwrote the existing file")
	}
	if _, statErr := os.Stat(filepath.Join(win64, PluginDirName)); !os.IsNotExist(statErr) {
		t.Error("install wrote the plugin despite refusing the loader conflict")
	}
}

func TestInspectReportsConflictForADifferentFile(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	if err := os.WriteFile(filepath.Join(win64, LoaderFileName), []byte("other-loader"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := writePayload(t, "loader-bytes", "plugin-bytes").Files

	// Act
	status, err := Inspect(win64, files)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateConflict {
		t.Fatalf("State = %q, want %q", status.State, StateConflict)
	}
	if status.Files[0].State != FileDifferent {
		t.Errorf("loader state = %q, want %q", status.Files[0].State, FileDifferent)
	}
}

func TestInstallRejectsPayloadThatFailsVerification(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	payload.Files[1].SHA256 = sha256Of(t, "some-other-content")

	// Act
	err := Install(win64, payload)

	// Assert
	if err == nil {
		t.Fatal("Install() succeeded with a payload that fails verification")
	}
	if _, statErr := os.Stat(filepath.Join(win64, LoaderFileName)); !os.IsNotExist(statErr) {
		t.Error("install wrote files before the payload passed verification")
	}
}

func TestInstallRollsBackWhenThePluginDirectoryCannotBeCreated(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	// A regular file at the plugins path makes MkdirAll fail after the
	// loader was already copied.
	blocker := filepath.Join(win64, PluginDirName)
	if err := os.WriteFile(blocker, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := writePayload(t, "loader-bytes", "plugin-bytes")

	// Act
	err := Install(win64, payload)

	// Assert
	if err == nil {
		t.Fatal("Install() succeeded despite the blocked plugin directory")
	}
	if _, statErr := os.Stat(filepath.Join(win64, LoaderFileName)); !os.IsNotExist(statErr) {
		t.Error("failed install left the loader behind instead of rolling it back")
	}
	got, readErr := os.ReadFile(blocker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "not-a-directory" {
		t.Error("failed install changed the file blocking the plugin directory")
	}
}

func TestRemoveDeletesOnlyCratebugFilesAndKeepsThePluginDirInUse(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	if err := Install(win64, payload); err != nil {
		t.Fatal(err)
	}
	otherPlugin := filepath.Join(win64, PluginDirName, "UnmountBlocker.asi")
	if err := os.WriteFile(otherPlugin, []byte("other-mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := Remove(win64, payload.Files)

	// Assert
	if err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(win64, LoaderFileName)); !os.IsNotExist(statErr) {
		t.Error("loader still present after remove")
	}
	if _, statErr := os.Stat(otherPlugin); statErr != nil {
		t.Error("remove deleted another mod's plugin")
	}
	status, err := Inspect(win64, payload.Files)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateNotInstalled {
		t.Fatalf("State = %q, want %q", status.State, StateNotInstalled)
	}
}

func TestRemoveDeletesAnEmptyPluginDirectory(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	if err := Install(win64, payload); err != nil {
		t.Fatal(err)
	}

	// Act
	err := Remove(win64, payload.Files)

	// Assert
	if err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(win64, PluginDirName)); !os.IsNotExist(statErr) {
		t.Error("empty plugin directory still present after remove")
	}
}

func TestRemoveRefusesWhenADifferentFileOccupiesAPayloadPath(t *testing.T) {
	// Arrange
	win64 := writeGameInstall(t)
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	if err := Install(win64, payload); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(win64, LoaderFileName), []byte("replaced-loader"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := Remove(win64, payload.Files)

	// Assert
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Remove() = %v, want ErrConflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(win64, LoaderFileName)); statErr != nil {
		t.Error("remove deleted a file it does not own")
	}
	if _, statErr := os.Stat(filepath.Join(win64, PluginDirName, PluginFileName)); statErr != nil {
		t.Error("remove deleted other payload files despite the conflict")
	}
}

func TestResolvePayloadDirectoryPrefersTheEnvironmentOverride(t *testing.T) {
	// Arrange
	envDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(envDir, LoaderFileName), []byte("loader"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := resolvePayloadDirectory(envDir, nil, nil, func(path string) bool {
		_, statErr := os.Stat(path)
		return statErr == nil
	})

	// Assert
	if err != nil {
		t.Fatalf("resolvePayloadDirectory() = %v", err)
	}
	if got != filepath.Clean(envDir) {
		t.Errorf("resolved %q, want %q", got, filepath.Clean(envDir))
	}
}

func TestResolvePayloadDirectoryRejectsAnOverrideWithoutTheLoader(t *testing.T) {
	// Act
	_, err := resolvePayloadDirectory(t.TempDir(), nil, nil, func(string) bool { return false })

	// Assert
	if !errors.Is(err, ErrPayloadNotFound) {
		t.Fatalf("resolvePayloadDirectory() = %v, want ErrPayloadNotFound", err)
	}
}

func TestResolvePayloadDirectoryFallsBackToProductionThenDevelopment(t *testing.T) {
	// Arrange
	exeDir := t.TempDir()
	prodDir := filepath.Join(exeDir, PayloadDevelopmentDirectory)
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prodDir, LoaderFileName), []byte("loader"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileExists := func(path string) bool {
		_, statErr := os.Stat(path)
		return statErr == nil
	}

	// Act
	got, err := resolvePayloadDirectory("", func() (string, error) {
		return filepath.Join(exeDir, "Cratebug.exe"), nil
	}, func() (string, error) { return t.TempDir(), nil }, fileExists)

	// Assert
	if err != nil {
		t.Fatalf("resolvePayloadDirectory() = %v", err)
	}
	if got != filepath.Clean(prodDir) {
		t.Errorf("resolved %q, want %q", got, filepath.Clean(prodDir))
	}

	// Act: no production payload, the working directory carries one.
	cwd := t.TempDir()
	devDir := filepath.Join(cwd, "build", PayloadDevelopmentDirectory)
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, LoaderFileName), []byte("loader"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = resolvePayloadDirectory("", func() (string, error) { return "", errors.New("no executable") }, func() (string, error) { return cwd, nil }, fileExists)

	// Assert
	if err != nil {
		t.Fatalf("resolvePayloadDirectory() = %v", err)
	}
	if got != filepath.Clean(devDir) {
		t.Errorf("resolved %q, want %q", got, filepath.Clean(devDir))
	}
}

func TestPayloadVerifyRejectsAChangedFile(t *testing.T) {
	// Arrange
	payload := writePayload(t, "loader-bytes", "plugin-bytes")
	if err := os.WriteFile(filepath.Join(payload.Dir, LoaderFileName), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	err := payload.Verify()

	// Assert
	if err == nil {
		t.Fatal("Verify() accepted a payload that no longer matches its pinned checksum")
	}
}

// Confirms the pinned digests still describe the exact release files. Runs
// only when the payload was fetched into build/sigbypass.
func TestPinnedPayloadMatchesTheFetchedRelease(t *testing.T) {
	// Arrange: tests run with the package directory as the working directory,
	// so point the resolver at the repository's fetched payload.
	t.Setenv(EnvPayloadPath, filepath.Join("..", "..", "build", PayloadDevelopmentDirectory))
	payload, err := PinnedPayload()
	if err != nil {
		t.Skipf("pinned payload not present; run fetch-sigbypass.ps1 first (see docs/decisions/0011-signature-bypass-tool.md)")
	}

	// Act
	err = payload.Verify()

	// Assert
	if err != nil {
		t.Fatalf("pinned payload does not match its digests: %v", err)
	}
}
