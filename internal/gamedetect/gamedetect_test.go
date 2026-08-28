package gamedetect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureDirPerm and fixtureFilePerm are the permissions used for every
// disposable fixture directory and file: the same shapes the real
// directories and a private file would have, with no bearing on test
// outcomes.
const fixtureDirPerm os.FileMode = 0o755
const fixtureFilePerm os.FileMode = 0o600

type stubProvider struct {
	name      string
	detection Detection
}

func (p stubProvider) Name() string {
	return p.name
}

func (p stubProvider) Detect() (Detection, error) {
	return p.detection, nil
}

// writeSteamVDF writes a libraryfolders.vdf beneath a fixture Steam root
// listing libraries the way Steam escapes them: %q doubles each path
// separator, matching the file's own format.
func writeSteamVDF(t *testing.T, steamRoot string, libraries ...string) {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("\"libraryfolders\"\n{\n")
	for _, library := range libraries {
		builder.WriteString(fmt.Sprintf("    \"path\"    %q\n", library))
	}
	builder.WriteString("}\n")

	vdfPath := filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf")
	if err := os.MkdirAll(filepath.Dir(vdfPath), fixtureDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vdfPath, []byte(builder.String()), fixtureFilePerm); err != nil {
		t.Fatal(err)
	}
}

// writeMarvelRivalsInstall creates the game-install directory shape beneath
// a fixture Steam library, optionally with its mod library folder, and
// returns the Paks path.
func writeMarvelRivalsInstall(t *testing.T, libraryRoot string, withLibrary bool) string {
	t.Helper()
	paksPath := filepath.Join(libraryRoot, "steamapps", "common", "MarvelRivals", "MarvelGame", "Marvel", "Content", "Paks")
	if err := os.MkdirAll(paksPath, fixtureDirPerm); err != nil {
		t.Fatal(err)
	}
	if withLibrary {
		if err := os.Mkdir(filepath.Join(paksPath, LibraryDirName), fixtureDirPerm); err != nil {
			t.Fatal(err)
		}
	}
	return paksPath
}

func TestParseSteamLibraryPaths(t *testing.T) {
	// Arrange
	content := strings.Join([]string{
		`"libraryfolders"`,
		`{`,
		`    "0"`,
		`    {`,
		`        "path"        "C:\\Program Files (x86)\\Steam"`,
		`        "label"       ""`,
		`        "apps"`,
		`        {`,
		`            "3124740"        "123456"`,
		`        }`,
		`    }`,
		`    "1"`,
		`    {`,
		`        "path"        "D:\\SteamLibrary"`,
		`    }`,
		`    "2"`,
		`    {`,
		`        "path"        "D:\\SteamLibrary"`,
		`    }`,
		`}`,
	}, "\n")

	// Act
	paths := parseSteamLibraryPaths(content)

	// Assert
	want := []string{`C:\Program Files (x86)\Steam`, `D:\SteamLibrary`}
	if len(paths) != len(want) {
		t.Fatalf("parseSteamLibraryPaths() = %v, want %v", paths, want)
	}
	for i, path := range want {
		if paths[i] != path {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], path)
		}
	}
}

func TestParseSteamLibraryPathsHandlesUnseparatedQuotes(t *testing.T) {
	// Arrange: some writers omit whitespace between the key and the value.
	content := "\"path\"\"E:\\Steam\"\n"

	// Act
	paths := parseSteamLibraryPaths(content)

	// Assert
	if len(paths) != 1 || paths[0] != `E:\Steam` {
		t.Fatalf("parseSteamLibraryPaths() = %v, want [E:\\Steam]", paths)
	}
}

func TestSteamProviderDetectFindsAnExistingLibrary(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	libraryRoot := t.TempDir()
	writeSteamVDF(t, steamRoot, libraryRoot)
	paksPath := writeMarvelRivalsInstall(t, libraryRoot, true)
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return steamRoot, nil },
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection.State != StateLibraryFound {
		t.Fatalf("State = %q, want %q", detection.State, StateLibraryFound)
	}
	if detection.PaksPath != paksPath {
		t.Errorf("PaksPath = %q, want %q", detection.PaksPath, paksPath)
	}
	if want := filepath.Join(paksPath, LibraryDirName); detection.LibraryPath != want {
		t.Errorf("LibraryPath = %q, want %q", detection.LibraryPath, want)
	}
}

func TestSteamProviderDetectReportsAnInstallWithoutALibrary(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	libraryRoot := t.TempDir()
	writeSteamVDF(t, steamRoot, libraryRoot)
	paksPath := writeMarvelRivalsInstall(t, libraryRoot, false)
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return steamRoot, nil },
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection.State != StateInstallFound {
		t.Fatalf("State = %q, want %q", detection.State, StateInstallFound)
	}
	if detection.PaksPath != paksPath {
		t.Errorf("PaksPath = %q, want %q", detection.PaksPath, paksPath)
	}
	if detection.LibraryPath != "" {
		t.Errorf("LibraryPath = %q, want empty while the library is missing", detection.LibraryPath)
	}
}

func TestSteamProviderDetectReportsNotFoundWithoutAnInstall(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	writeSteamVDF(t, steamRoot, t.TempDir())
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return steamRoot, nil },
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection.State != StateNotFound {
		t.Fatalf("State = %q, want %q", detection.State, StateNotFound)
	}
	if detection.PaksPath != "" || detection.LibraryPath != "" {
		t.Errorf("Detection = %+v, want empty paths for a not-found result", detection)
	}
}

func TestSteamProviderDetectPrefersTheFirstLibraryInOrder(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	firstLibrary := t.TempDir()
	secondLibrary := t.TempDir()
	writeSteamVDF(t, steamRoot, firstLibrary, secondLibrary)
	writeMarvelRivalsInstall(t, firstLibrary, true)
	writeMarvelRivalsInstall(t, secondLibrary, true)
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return steamRoot, nil },
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(firstLibrary, "steamapps", "common", "MarvelRivals", "MarvelGame", "Marvel", "Content", "Paks"); detection.PaksPath != want {
		t.Errorf("PaksPath = %q, want the first library's %q", detection.PaksPath, want)
	}
}

func TestSteamProviderDetectIgnoresAFileShapedLikeThePaksDirectory(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	libraryRoot := t.TempDir()
	writeSteamVDF(t, steamRoot, libraryRoot)
	paksPath := filepath.Join(libraryRoot, "steamapps", "common", "MarvelRivals", "MarvelGame", "Marvel", "Content", "Paks")
	if err := os.MkdirAll(filepath.Dir(paksPath), fixtureDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paksPath, []byte("not a directory"), fixtureFilePerm); err != nil {
		t.Fatal(err)
	}
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return steamRoot, nil },
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection.State != StateNotFound {
		t.Fatalf("State = %q, want %q for a file pretending to be the Paks directory", detection.State, StateNotFound)
	}
}

func TestSteamProviderDetectFallsBackWhenTheRegistryHasNoAnswer(t *testing.T) {
	// Arrange
	steamRoot := t.TempDir()
	libraryRoot := t.TempDir()
	writeSteamVDF(t, steamRoot, libraryRoot)
	writeMarvelRivalsInstall(t, libraryRoot, true)
	provider := SteamProvider{
		registrySteamPath: func() (string, error) { return "", errors.New("registry unavailable") },
		fallbackRoots:     []string{steamRoot},
	}

	// Act
	detection, err := provider.Detect()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detection.State != StateLibraryFound {
		t.Fatalf("State = %q, want %q", detection.State, StateLibraryFound)
	}
}

func TestRegistryDetectRejectsAnUnknownProvider(t *testing.T) {
	// Arrange
	registry := NewRegistry(stubProvider{name: ProviderSteam})

	// Act
	_, err := registry.Detect("epic")

	// Assert
	if err == nil {
		t.Fatal("Detect() succeeded for an unknown provider, want an error")
	}
}

func TestRegistryCreateLibraryCreatesOnlyTheMissingFolder(t *testing.T) {
	// Arrange
	paksPath := t.TempDir()
	registry := NewRegistry(stubProvider{
		name:      ProviderSteam,
		detection: Detection{State: StateInstallFound, PaksPath: paksPath},
	})

	// Act
	libraryPath, err := registry.CreateLibrary(ProviderSteam)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(paksPath, LibraryDirName); libraryPath != want {
		t.Fatalf("CreateLibrary() = %q, want %q", libraryPath, want)
	}
	if !isDir(libraryPath) {
		t.Errorf("the mod library directory %q was not created", libraryPath)
	}
	entries, err := os.ReadDir(paksPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != LibraryDirName {
		t.Errorf("Paks directory contains %v, want only %q", entries, LibraryDirName)
	}
}

func TestRegistryCreateLibraryReturnsAnExistingLibraryUnchanged(t *testing.T) {
	// Arrange
	paksPath := t.TempDir()
	libraryPath := filepath.Join(paksPath, LibraryDirName)
	if err := os.Mkdir(libraryPath, fixtureDirPerm); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(stubProvider{
		name:      ProviderSteam,
		detection: Detection{State: StateLibraryFound, LibraryPath: libraryPath, PaksPath: paksPath},
	})

	// Act
	got, err := registry.CreateLibrary(ProviderSteam)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got != libraryPath {
		t.Errorf("CreateLibrary() = %q, want the existing %q", got, libraryPath)
	}
}

func TestRegistryCreateLibraryRefusesWithoutAnInstall(t *testing.T) {
	// Arrange
	registry := NewRegistry(stubProvider{
		name:      ProviderSteam,
		detection: Detection{State: StateNotFound},
	})

	// Act
	_, err := registry.CreateLibrary(ProviderSteam)

	// Assert
	if err == nil {
		t.Fatal("CreateLibrary() succeeded without an install, want an error")
	}
}

func TestValidProvider(t *testing.T) {
	// Arrange / Act / Assert
	if !ValidProvider(ProviderSteam) {
		t.Errorf("ValidProvider(%q) = false, want true", ProviderSteam)
	}
	if ValidProvider("epic") {
		t.Error("ValidProvider(\"epic\") = true, want false while only Steam ships")
	}
	if ValidProvider("") {
		t.Error("ValidProvider(\"\") = true, want false")
	}
}
