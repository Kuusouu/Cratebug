// Package sigbypass installs the pinned UTOC signature bypass payload into a
// detected Marvel Rivals installation. The payload lets the game load modded
// pak/utoc/ucas files without matching .sig files: Ultimate ASI Loader (MIT,
// ThirteenAG) renamed to dsound.dll, plus DeathChaos25's bypass plugin. The
// release build bundles the payload next to Cratebug's executable; this
// package copies it into the game's Win64 directory and removes it again.
package sigbypass

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// The shipping executable that must exist in an install's Win64
	// directory for the payload to load and for the game to be considered
	// installed.
	GameExecutableName = "Marvel-Win64-Shipping.exe"

	// Ultimate ASI Loader, renamed to dsound.dll so the game loads it.
	LoaderFileName = "dsound.dll"

	// The plugin directory Ultimate ASI Loader scans next to itself. Other
	// ASI mods live there too, so Cratebug only ever adds or removes its
	// own plugin file.
	PluginDirName = "plugins"

	// The pinned UTOC signature bypass plugin.
	PluginFileName = "MarvelRivalsUTOCSignatureBypass.asi"

	// Overrides where the bundled payload is located.
	EnvPayloadPath = "CRATEBUG_SIGBYPASS_PATH"

	// The fetched payload directory under build/ that development runs and
	// tests resolve.
	PayloadDevelopmentDirectory = "sigbypass"

	installedFilePerm os.FileMode = 0o644
	installedDirPerm  os.FileMode = 0o755
)

// Reported when no Marvel Rivals installation can be located for the bypass.
var ErrGameNotFound = errors.New("sigbypass: Marvel Rivals installation not found")

// Reported when the bundled payload cannot be resolved or read.
var ErrPayloadNotFound = errors.New("sigbypass: signature bypass payload not found")

// Reported when a file Cratebug does not own occupies an install path.
// Nothing is overwritten or removed; the caller reports the conflict.
var ErrConflict = errors.New("sigbypass: a different file occupies an install path")

// Describes one payload file's state inside a game installation.
type FileState string

const (
	// The file is absent from the game installation.
	FileMissing FileState = "missing"
	// The file matches the pinned payload checksum.
	FileCurrent FileState = "current"
	// A file exists there but is not the pinned payload.
	FileDifferent FileState = "different"
)

// Describes the overall bypass state of one game installation.
type State string

const (
	// No game installation could be detected.
	StateGameNotFound State = "gameNotFound"
	// No payload file is present.
	StateNotInstalled State = "notInstalled"
	// Every payload file is present and current.
	StateInstalled State = "installed"
	// Some payload files are present and none differ.
	StatePartial State = "partial"
	// At least one path holds a different file.
	StateConflict State = "conflict"
)

// Pairs one payload path with its current state.
type FileStatus struct {
	RelativePath string    `json:"relativePath"`
	State        FileState `json:"state"`
}

// Reports the bypass state of one game installation. GameDir is set
// whenever an installation was detected.
type Status struct {
	State            State        `json:"state"`
	GameDir          string       `json:"gameDir,omitempty"`
	PayloadAvailable bool         `json:"payloadAvailable"`
	Files            []FileStatus `json:"files,omitempty"`
}

// One payload file and the SHA-256 it must have.
type PayloadFile struct {
	RelativePath string
	SHA256       string
}

// A locally resolved payload directory and its file list.
type Payload struct {
	Dir   string
	Files []PayloadFile
}

// The files the bypass writes into a game installation, with the digests
// pinned in fetch-sigbypass.ps1 and
// docs/decisions/0011-signature-bypass-tool.md. The order keeps status
// output stable.
var installedFiles = []PayloadFile{
	{RelativePath: LoaderFileName, SHA256: "bb8767f918c52a2ad055d2de9baffd2478598643b9894f09abd20d1f1ffd170c"},
	{RelativePath: filepath.Join(PluginDirName, PluginFileName), SHA256: "2f8b183149f30c94319a5f6636491ff39aa22afbbf3f5cbc5fb8f7ecc287a62d"},
}

// Derives the Win64 directory that sits beside a detected installation's
// Content/Paks directory and verifies the game executable is there, so the
// payload is only ever aimed at an install that provider detection just
// verified.
func GameDirForPaks(paksPath string) (string, error) {
	if strings.TrimSpace(paksPath) == "" {
		return "", ErrGameNotFound
	}

	gameDir := filepath.Clean(filepath.Join(paksPath, "..", "..", "Binaries", "Win64"))
	if err := verifyGameDir(gameDir); err != nil {
		return "", err
	}
	return gameDir, nil
}

// Reads the current state of files in a game installation against the
// expected payload file list. Read-only: nothing is written for any
// outcome.
func Inspect(gameDir string, files []PayloadFile) (Status, error) {
	status := Status{State: StateNotInstalled, GameDir: gameDir, Files: make([]FileStatus, 0, len(files))}
	present := false
	current := true

	for _, file := range files {
		state, err := inspectFile(filepath.Join(gameDir, file.RelativePath), file.SHA256)
		if err != nil {
			return Status{}, err
		}
		status.Files = append(status.Files, FileStatus{RelativePath: file.RelativePath, State: state})
		switch state {
		case FileCurrent:
			present = true
		case FileDifferent:
			present = true
			current = false
		default:
			current = false
		}
	}

	switch {
	case !present:
		status.State = StateNotInstalled
	case !current:
		status.State = stateForMixed(status.Files)
	default:
		status.State = StateInstalled
	}
	return status, nil
}

// Copies the verified payload into the game installation. Any pre-existing
// file that is not part of the pinned payload is a conflict: the install is
// refused before anything is written, never overwritten. Files already
// current are left alone, so a partial install is repaired.
func Install(gameDir string, payload Payload) error {
	if err := verifyGameDir(gameDir); err != nil {
		return err
	}
	if err := payload.Verify(); err != nil {
		return err
	}
	if err := planInstall(gameDir, payload.Files); err != nil {
		return err
	}

	var created []string
	fail := func(installErr error) error {
		var cleanupErr error
		for i := len(created) - 1; i >= 0; i-- {
			if err := os.Remove(created[i]); err != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("roll back %s: %w", created[i], err))
			}
		}
		if cleanupErr != nil {
			return errors.Join(installErr, cleanupErr)
		}
		return installErr
	}

	for _, file := range payload.Files {
		target := filepath.Join(gameDir, file.RelativePath)
		if state, err := inspectFile(target, file.SHA256); err != nil {
			return fail(err)
		} else if state == FileCurrent {
			continue
		}
		// The plan pass saw no target; refuse anything that appeared since.
		if _, err := os.Lstat(target); err == nil {
			return fail(fmt.Errorf("%w: %s", ErrConflict, target))
		}

		if err := os.MkdirAll(filepath.Dir(target), installedDirPerm); err != nil {
			return fail(fmt.Errorf("create %s: %w", filepath.Dir(target), err))
		}
		source := filepath.Join(payload.Dir, file.RelativePath)
		if err := copyFileVerified(source, target, file.SHA256); err != nil {
			return fail(err)
		}
		created = append(created, target)
	}
	return nil
}

// Deletes the expected payload files from the game installation. A
// different file at any payload path refuses the removal untouched, and the
// plugin directory is only removed when it ends up empty, so other ASI mods
// keep working.
func Remove(gameDir string, files []PayloadFile) error {
	if err := verifyGameDir(gameDir); err != nil {
		return err
	}
	if err := planRemove(gameDir, files); err != nil {
		return err
	}

	for _, file := range files {
		target := filepath.Join(gameDir, file.RelativePath)
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", target, err)
		}
	}
	// Best effort: a plugin directory that still holds other ASI mods stays.
	_ = os.Remove(filepath.Join(gameDir, PluginDirName))
	return nil
}

// Returns the pinned file list used to inspect game installations and to
// remove installed files. It carries the same digests as the payload that
// installs them.
func InstalledFiles() []PayloadFile {
	return slices.Clone(installedFiles)
}

// Resolves the bundled payload directory and pairs it with the pinned
// digests. Callers only ever install a payload that passed verification.
func PinnedPayload() (Payload, error) {
	dir, err := ResolvePayloadDirectory()
	if err != nil {
		return Payload{}, err
	}
	return Payload{Dir: dir, Files: slices.Clone(installedFiles)}, nil
}

// Locates the bundled payload by checking in order:
//  1. The CRATEBUG_SIGBYPASS_PATH environment variable override.
//  2. The installed production layout: <executable-dir>/sigbypass/.
//  3. The development layout under build/sigbypass/.
func ResolvePayloadDirectory() (string, error) {
	return resolvePayloadDirectory(
		os.Getenv(EnvPayloadPath),
		os.Executable,
		os.Getwd,
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	)
}

func resolvePayloadDirectory(
	envPath string,
	executablePathFunc func() (string, error),
	getwdFunc func() (string, error),
	fileExists func(string) bool,
) (string, error) {
	hasLoader := func(dir string) bool {
		return fileExists(filepath.Join(dir, LoaderFileName))
	}

	if envPath != "" {
		cleaned := filepath.Clean(envPath)
		if hasLoader(cleaned) {
			return cleaned, nil
		}
		return "", fmt.Errorf("%w: %s does not contain %s", ErrPayloadNotFound, cleaned, LoaderFileName)
	}

	if executablePathFunc != nil {
		if exe, err := executablePathFunc(); err == nil && exe != "" {
			prodDir := filepath.Join(filepath.Dir(exe), PayloadDevelopmentDirectory)
			if hasLoader(prodDir) {
				return filepath.Clean(prodDir), nil
			}
		}
	}

	if getwdFunc != nil {
		if cwd, err := getwdFunc(); err == nil && cwd != "" {
			devDir := filepath.Join(cwd, "build", PayloadDevelopmentDirectory)
			if hasLoader(devDir) {
				return filepath.Clean(devDir), nil
			}
		}
	}

	relDevDir := filepath.Join("build", PayloadDevelopmentDirectory)
	if hasLoader(relDevDir) {
		return filepath.Clean(relDevDir), nil
	}

	return "", fmt.Errorf("%w: checked %s, production layout (<exe>/%s), and development layout (build/%s)",
		ErrPayloadNotFound, EnvPayloadPath, PayloadDevelopmentDirectory, PayloadDevelopmentDirectory)
}

// Confirms every payload file exists and matches its pinned digest.
func (p Payload) Verify() error {
	for _, file := range p.Files {
		source := filepath.Join(p.Dir, file.RelativePath)
		sum, err := fileSHA256(source)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrPayloadNotFound, source)
			}
			return fmt.Errorf("read payload file %s: %w", source, err)
		}
		if sum != file.SHA256 {
			return fmt.Errorf("payload file %s does not match its pinned checksum", source)
		}
	}
	return nil
}

func stateForMixed(files []FileStatus) State {
	for _, file := range files {
		if file.State == FileDifferent {
			return StateConflict
		}
	}
	return StatePartial
}

func inspectFile(path, wantSHA256 string) (FileState, error) {
	sum, err := fileSHA256(path)
	switch {
	case os.IsNotExist(err):
		return FileMissing, nil
	case err != nil:
		return "", fmt.Errorf("inspect %s: %w", path, err)
	case sum == wantSHA256:
		return FileCurrent, nil
	default:
		return FileDifferent, nil
	}
}

func planInstall(gameDir string, files []PayloadFile) error {
	for _, file := range files {
		target := filepath.Join(gameDir, file.RelativePath)
		state, err := inspectFile(target, file.SHA256)
		if err != nil {
			return err
		}
		if state == FileDifferent {
			return fmt.Errorf("%w: %s", ErrConflict, target)
		}
	}
	return nil
}

func planRemove(gameDir string, files []PayloadFile) error {
	var conflicts []string
	for _, file := range files {
		target := filepath.Join(gameDir, file.RelativePath)
		state, err := inspectFile(target, file.SHA256)
		if err != nil {
			return err
		}
		if state == FileDifferent {
			conflicts = append(conflicts, target)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w: %s", ErrConflict, strings.Join(conflicts, ", "))
	}
	return nil
}

func verifyGameDir(gameDir string) error {
	exe := filepath.Join(gameDir, GameExecutableName)
	if info, err := os.Stat(exe); err != nil || info.IsDir() {
		return fmt.Errorf("%w: %s does not contain %s", ErrGameNotFound, gameDir, GameExecutableName)
	}
	return nil
}

// Writes source to target through a temporary file in the target directory,
// refusing to publish anything whose bytes do not hash to the expected
// digest. The rename keeps a truncated copy from ever appearing at the
// target path.
func copyFileVerified(source, target, wantSHA256 string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open payload file %s: %w", source, err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(target), ".cratebug-sigbypass-*")
	if err != nil {
		return fmt.Errorf("create temporary file next to %s: %w", target, err)
	}
	tmpPath := tmp.Name()
	cleanup := func(copyErr error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return copyErr
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), in); err != nil {
		return cleanup(fmt.Errorf("copy %s to %s: %w", source, target, err))
	}
	if err := tmp.Close(); err != nil {
		return cleanup(fmt.Errorf("close temporary file for %s: %w", target, err))
	}
	if actual := hex.EncodeToString(hasher.Sum(nil)); actual != wantSHA256 {
		return cleanup(fmt.Errorf("payload file %s changed while copying; got checksum %s", source, actual))
	}
	if err := os.Chmod(tmpPath, installedFilePerm); err != nil {
		return cleanup(fmt.Errorf("set permissions on %s: %w", target, err))
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return cleanup(fmt.Errorf("place %s: %w", target, err))
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
