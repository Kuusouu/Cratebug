package backup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kuusouu/Cratebug/internal/discovery"
	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/metadata"
)

const (
	// Random bytes per restore session token. Mirrors the install session
	// manager so tokens are unguessable across both flows.
	sessionTokenEntropyBytes = 16

	// Only backups produced by the backup flow are restorable. Anything
	// else is rejected with a plain error before staging begins.
	restorableExtension = ".zip"
)

// Describes staged restore contents without changing anything. Counts come
// from a live scan of the staged tree, never from anything stored in the
// archive; ZipModified is the archive file's own modification time, shown
// labeled as such in the confirm preview. A dismissed open dialog reports
// Cancelled instead of an error so the frontend can stay silent.
type Preview struct {
	Token           string    `json:"token"`
	Counts          Counts    `json:"counts"`
	MetadataPresent bool      `json:"metadataPresent"`
	ZipModified     time.Time `json:"zipModified"`
	Cancelled       bool      `json:"cancelled"`
}

// Describes a completed restore. Counts come from a post-restore scan of
// the library. MetadataRestored reports whether the staged metadata replaced
// the current file; MetadataNote explains why not when it did not.
type RestoreResult struct {
	Counts           Counts `json:"counts"`
	MetadataRestored bool   `json:"metadataRestored"`
	MetadataNote     string `json:"metadataNote,omitempty"`
}

// Holds one staged restore between its preview and its apply. The staging
// directory keeps the validated tree; StagedMetadata holds the raw metadata
// bytes when the archive carried them.
type restoreSession struct {
	stagingDir     string
	stagedMetadata []byte
	metadataFound  bool
}

// Coordinates staged restores across calls. Sessions own their staging
// directories until applied or discarded.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*restoreSession
}

// Creates a manager with no live sessions.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*restoreSession)}
}

// Stages zipPath for a later apply: extracts with install-grade traversal
// protection, separates the metadata entry, and scans the staged tree.
// Nothing outside the staging directory is touched. The returned session
// lives in the manager until Apply or DiscardSession runs.
func (m *SessionManager) Preview(ctx context.Context, zipPath string) (Preview, error) {
	if !strings.EqualFold(filepath.Ext(zipPath), restorableExtension) {
		return Preview{}, fmt.Errorf("restore expects a .zip backup, got %q", zipPath)
	}
	info, err := os.Stat(zipPath)
	if err != nil {
		return Preview{}, fmt.Errorf("stat backup archive: %w", err)
	}
	if info.IsDir() {
		return Preview{}, fmt.Errorf("backup archive is a directory: %q", zipPath)
	}

	stagingDir, err := os.MkdirTemp("", "cratebug-restore-*")
	if err != nil {
		return Preview{}, fmt.Errorf("create restore staging directory: %w", err)
	}
	treeDir := filepath.Join(stagingDir, "tree")
	if err := os.Mkdir(treeDir, 0o700); err != nil {
		_ = os.RemoveAll(stagingDir)
		return Preview{}, fmt.Errorf("create restore staging tree: %w", err)
	}

	if err := install.ExtractArchive(ctx, zipPath, treeDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return Preview{}, err
	}

	metadataBytes, metadataFound, err := separateMetadata(treeDir)
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return Preview{}, err
	}

	library, err := discovery.Scan(treeDir)
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return Preview{}, fmt.Errorf("scan staged backup: %w", err)
	}

	token, err := newSessionToken()
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return Preview{}, err
	}
	m.mu.Lock()
	m.sessions[token] = &restoreSession{
		stagingDir:     stagingDir,
		stagedMetadata: metadataBytes,
		metadataFound:  metadataFound,
	}
	m.mu.Unlock()

	return Preview{
		Token:           token,
		Counts:          CountsLibrary(library.Entries),
		MetadataPresent: metadataFound,
		ZipModified:     info.ModTime(),
	}, nil
}

// Drops a staged session and its staging directory without touching the library.
func (m *SessionManager) DiscardSession(token string) {
	m.mu.Lock()
	session, ok := m.sessions[token]
	if ok {
		delete(m.sessions, token)
	}
	m.mu.Unlock()
	if ok {
		_ = os.RemoveAll(session.stagingDir)
	}
}

// Replaces the mod-root tree with the staged session's contents and restores
// its metadata through the store's safe-write path. The current tree is
// parked aside first; any failure moves it back, so a failed restore never
// presents a partial library. The session is consumed whether apply succeeds
// or fails. Cancellation aborts with the previous library put back.
func (m *SessionManager) Apply(ctx context.Context, modRoot, metadataPath, token string, onProgress func(Progress)) (RestoreResult, error) {
	m.mu.Lock()
	session, ok := m.sessions[token]
	if ok {
		delete(m.sessions, token)
	}
	m.mu.Unlock()
	if !ok {
		return RestoreResult{}, fmt.Errorf("restore session not found or already applied")
	}
	defer os.RemoveAll(session.stagingDir)

	if err := ctx.Err(); err != nil {
		return RestoreResult{}, fmt.Errorf("restore cancelled before applying: %w", err)
	}

	treeDir := filepath.Join(session.stagingDir, "tree")
	root, err := filepath.Abs(modRoot)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("resolve mod library path: %w", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return RestoreResult{}, fmt.Errorf("mod library is not a directory: %q", modRoot)
	}

	// A leftover park means a previous restore died mid-apply. Refuse
	// rather than guess which side is authoritative.
	parkDir := root + ".restore-park"
	if _, err := os.Lstat(parkDir); err == nil {
		return RestoreResult{}, fmt.Errorf("a previous restore park still exists at %q; remove it before restoring", parkDir)
	} else if !os.IsNotExist(err) {
		return RestoreResult{}, fmt.Errorf("inspect restore park location: %w", err)
	}
	if err := os.Mkdir(parkDir, 0o700); err != nil {
		return RestoreResult{}, fmt.Errorf("create restore park: %w", err)
	}
	parked, placed, parkErr := parkAndPlace(ctx, root, parkDir, treeDir, onProgress)
	if parkErr != nil {
		rollbackErr := rollbackPark(root, parkDir, treeDir, parked, placed)
		_ = os.RemoveAll(parkDir)
		if rollbackErr != nil {
			return RestoreResult{}, fmt.Errorf("restore failed: %v; rollback also failed: %v", parkErr, rollbackErr)
		}
		return RestoreResult{}, parkErr
	}

	result := RestoreResult{}
	if session.metadataFound {
		restored, note, err := restoreMetadata(metadataPath, session.stagedMetadata)
		result.MetadataRestored = restored
		result.MetadataNote = note
		if err != nil {
			rollbackErr := rollbackPark(root, parkDir, treeDir, parked, placed)
			_ = os.RemoveAll(parkDir)
			if rollbackErr != nil {
				return RestoreResult{}, fmt.Errorf("restore metadata failed: %v; rollback also failed: %v", err, rollbackErr)
			}
			return RestoreResult{}, err
		}
	} else {
		result.MetadataNote = "backup holds no metadata; current metadata kept"
	}

	library, err := discovery.Scan(root)
	if err != nil {
		rollbackErr := rollbackPark(root, parkDir, treeDir, parked, placed)
		_ = os.RemoveAll(parkDir)
		if rollbackErr != nil {
			return RestoreResult{}, fmt.Errorf("verify restored library: %v; rollback also failed: %v", err, rollbackErr)
		}
		return RestoreResult{}, fmt.Errorf("verify restored library: %w", err)
	}
	_ = os.RemoveAll(parkDir)

	result.Counts = CountsLibrary(library.Entries)
	return result, nil
}

// Moves the staged metadata entry out of the extracted tree so it can never
// land in the library as a stray file. Reports whether the archive carried
// one; a missing entry is fine and leaves the current metadata untouched.
func separateMetadata(treeDir string) ([]byte, bool, error) {
	path := filepath.Join(treeDir, metadataEntryName)
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read staged metadata: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return nil, false, fmt.Errorf("separate staged metadata: %w", err)
	}
	return contents, true, nil
}

// Parses staged metadata and writes it through the store's safe-write path,
// which keeps the current file as a last-known-good backup. Content this
// build cannot understand keeps the current file instead of letting the next
// startup silently recover away from it.
func restoreMetadata(metadataPath string, staged []byte) (bool, string, error) {
	if metadataPath == "" {
		return false, "no metadata location configured; current metadata kept", nil
	}
	var doc metadata.Document
	if err := json.Unmarshal(staged, &doc); err != nil {
		return false, "backup metadata is not valid JSON; current metadata kept", nil
	}
	if doc.SchemaVersion < 0 || doc.SchemaVersion > metadata.CurrentSchemaVersion {
		return false, fmt.Sprintf("backup metadata uses schema version %d; current metadata kept", doc.SchemaVersion), nil
	}
	if err := metadata.NewStore(metadataPath).Save(doc); err != nil {
		return false, "", fmt.Errorf("write restored metadata: %w", err)
	}
	return true, "", nil
}

// Moves every current library child into the park, then moves every staged
// child into the library, reporting each move. Returns both name lists so a
// later failure can put everything back where it was.
func parkAndPlace(ctx context.Context, root, parkDir, treeDir string, onProgress func(Progress)) (parked, placed []string, err error) {
	current, err := readDirNames(root)
	if err != nil {
		return nil, nil, fmt.Errorf("list mod library: %w", err)
	}
	staged, err := readDirNames(treeDir)
	if err != nil {
		return nil, nil, fmt.Errorf("list staged backup: %w", err)
	}
	total := len(current) + len(staged)
	done := 0
	report := func(name string) {
		done++
		if onProgress != nil {
			onProgress(Progress{Current: done, Total: total, CurrentFile: name})
		}
	}

	for _, name := range current {
		if err := ctx.Err(); err != nil {
			return parked, nil, fmt.Errorf("restore cancelled while parking: %w", err)
		}
		if err := os.Rename(filepath.Join(root, name), filepath.Join(parkDir, name)); err != nil {
			return parked, nil, fmt.Errorf("park mod library entry %q: %w", name, err)
		}
		parked = append(parked, name)
		report(name)
	}

	for _, name := range staged {
		if err := ctx.Err(); err != nil {
			return parked, placed, fmt.Errorf("restore cancelled while placing: %w", err)
		}
		if err := os.Rename(filepath.Join(treeDir, name), filepath.Join(root, name)); err != nil {
			return parked, placed, fmt.Errorf("place restored entry %q: %w", name, err)
		}
		placed = append(placed, name)
		report(name)
	}
	return parked, placed, nil
}

// Returns placed entries to staging and parked entries to the library after
// a failed apply. Reports the first rollback failure, if any; callers
// surface it alongside the original error so a half-rolled-back library is
// never presented as merely failed.
func rollbackPark(root, parkDir, treeDir string, parked, placed []string) error {
	for i := len(placed) - 1; i >= 0; i-- {
		name := placed[i]
		if err := os.Rename(filepath.Join(root, name), filepath.Join(treeDir, name)); err != nil {
			return fmt.Errorf("return placed entry %q: %w", name, err)
		}
	}
	for _, name := range parked {
		if err := os.Rename(filepath.Join(parkDir, name), filepath.Join(root, name)); err != nil {
			return fmt.Errorf("restore parked entry %q: %w", name, err)
		}
	}
	return nil
}

// Lists immediate child names in deterministic order.
func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// Generates an unguessable session token.
func newSessionToken() (string, error) {
	var entropy [sessionTokenEntropyBytes]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate restore session token: %w", err)
	}
	return hex.EncodeToString(entropy[:]), nil
}
