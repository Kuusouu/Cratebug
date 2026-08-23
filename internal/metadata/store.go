// Package metadata persists Cratebug settings, tags, and mod identity across sessions.
package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CurrentSchemaVersion identifies the Document shape Save writes and Load reads.
const CurrentSchemaVersion = 1

const backupSuffix = ".bak"
const quarantineSuffix = ".corrupt"

// Recovery reports whether Load had to fall back to the last-known-good
// backup, and what was wrong with the file it moved aside. It is never
// persisted; callers use it to surface actionable feedback instead of
// silently losing track of a corruption event.
type Recovery struct {
	Recovered bool
	Cause     error
}

// Settings holds app-level preferences that persist across sessions.
type Settings struct {
	ModRoot         string `json:"modRoot,omitempty"`
	Theme           string `json:"theme,omitempty"`
	DefaultViewMode string `json:"defaultViewMode,omitempty"`
	AccentColor     string `json:"accentColor,omitempty"`
}

// Document is the versioned envelope persisted to disk.
type Document struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Settings      Settings             `json:"settings"`
	Mods          map[string]ModRecord `json:"mods,omitempty"`
	Tags          []Tag                `json:"tags,omitempty"`
}

// Store reads and writes one Document at a fixed filesystem path.
type Store struct {
	path string
}

// Creates a Store bound to path. Callers choose the path so tests can use
// disposable directories instead of the real per-user config location.
func NewStore(path string) Store {
	return Store{path: path}
}

// Returns the default per-user location for Cratebug's persisted metadata.
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "Cratebug", "metadata.json"), nil
}

// Loads the persisted document, or a fresh one at the current schema version
// if nothing has been saved yet.
//
// Load cannot fail: a missing, unreadable, malformed, or unsupported-schema
// primary file is quarantined (renamed aside, not discarded) and recovery
// falls back to the last-known-good backup, or to a fresh document if the
// backup is unusable too. A corrupt file must never block Cratebug from
// starting.
func (s Store) Load() (Document, Recovery) {
	doc, err := s.readDocument(s.path)
	if err == nil {
		return doc, Recovery{}
	}
	if os.IsNotExist(err) {
		return Document{SchemaVersion: CurrentSchemaVersion}, Recovery{}
	}

	// Best-effort: quarantining preserves the bad file for inspection, but a
	// failure here must not prevent falling back to the backup.
	_ = s.quarantinePrimary()

	backupDoc, backupErr := s.readDocument(s.path + backupSuffix)
	if backupErr != nil {
		return Document{SchemaVersion: CurrentSchemaVersion}, Recovery{Recovered: true, Cause: err}
	}

	// Self-heal: restore the primary from the backup so the next Load does
	// not have to repeat this recovery. A write failure here is not fatal;
	// the recovered document is still returned for the caller to use.
	_ = s.Save(backupDoc)
	return backupDoc, Recovery{Recovered: true, Cause: err}
}

// Reads one metadata file, migrates it to CurrentSchemaVersion if it was
// written by an older, supported schema version, and rejects a schema
// version this build does not understand: either newer than
// CurrentSchemaVersion, or older than every registered migration can reach.
func (s Store) readDocument(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Document{}, fmt.Errorf("parse metadata file %q: %w", path, err)
	}

	version := schemaVersionOf(raw)
	if version > CurrentSchemaVersion {
		return Document{}, fmt.Errorf("metadata file %q uses schema version %d, newer than the %d this build supports", path, version, CurrentSchemaVersion)
	}

	for version < CurrentSchemaVersion {
		migrate, ok := migrations[version]
		if !ok {
			return Document{}, fmt.Errorf("metadata file %q uses schema version %d, which this build cannot migrate to %d", path, version, CurrentSchemaVersion)
		}
		if err := migrate(raw); err != nil {
			return Document{}, fmt.Errorf("migrate metadata file %q from schema version %d: %w", path, version, err)
		}

		nextVersion := schemaVersionOf(raw)
		if nextVersion <= version {
			return Document{}, fmt.Errorf("migration from schema version %d for %q did not advance the schema version", version, path)
		}
		version = nextVersion
	}

	migrated, err := json.Marshal(raw)
	if err != nil {
		return Document{}, fmt.Errorf("re-encode migrated metadata file %q: %w", path, err)
	}

	var doc Document
	if err := json.Unmarshal(migrated, &doc); err != nil {
		return Document{}, fmt.Errorf("decode migrated metadata file %q: %w", path, err)
	}
	return doc, nil
}

// Moves an unreadable or invalid primary file aside instead of discarding
// it, so a corrupted file remains available for inspection or recovery.
func (s Store) quarantinePrimary() error {
	if err := os.Rename(s.path, s.path+quarantineSuffix); err != nil {
		return fmt.Errorf("quarantine metadata file: %w", err)
	}
	return nil
}

// Saves doc, stamping it with the current schema version.
//
// The write is atomic: a partial or interrupted write never modifies the file
// already at s.path. The file's previous contents, if any, are copied to a
// last-known-good backup before being replaced.
func (s Store) Save(doc Document) error {
	doc.SchemaVersion = CurrentSchemaVersion

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create metadata directory: %w", err)
	}

	if err := s.updateBackup(); err != nil {
		return fmt.Errorf("update metadata backup: %w", err)
	}

	if err := writeFileAtomically(dir, s.path, data); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}
	return nil
}

// Copies the file currently at s.path to its backup path before Save replaces
// it, so a later corrupt or unreadable primary file still has a known-good
// fallback to recover from.
func (s Store) updateBackup() error {
	current, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current metadata file: %w", err)
	}
	return writeFileAtomically(filepath.Dir(s.path), s.path+backupSuffix, current)
}

// Writes data to destination by creating a temporary file in dir and renaming
// it into place, so a failure partway through writing never leaves
// destination in a partially written state.
func writeFileAtomically(dir, destination string, data []byte) error {
	temp, err := os.CreateTemp(dir, filepath.Base(destination)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tempPath, destination); err != nil {
		return fmt.Errorf("rename temporary file into place: %w", err)
	}
	return nil
}
