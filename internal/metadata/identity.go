package metadata

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const modIDPrefix = "mod-"

// ModRecord is the persisted state Cratebug attaches to one mod, keyed by a
// persistent identity that survives rename, priority change, and move.
//
// ScannerID is the current scanner-derived identity (discovery.Entry.ID),
// which changes whenever the mod's folder or filename stem changes. A record
// whose ScannerID no longer matches any entry in a fresh scan is orphaned:
// task 5.4 detects and surfaces that case instead of discarding the record,
// since the mod may reappear (for example, after reselecting a folder).
type ModRecord struct {
	ScannerID string   `json:"scannerID"`
	Tags      []string `json:"tags,omitempty"`
}

// Returns the persistent identity for scannerID, assigning a new one the
// first time this scanner ID is seen. Same-named mods in different folders
// have different scanner IDs and therefore receive independent identities.
func (doc *Document) EnsureMod(scannerID string) (string, error) {
	if id, ok := doc.FindModByScannerID(scannerID); ok {
		return id, nil
	}

	id, err := newID(modIDPrefix)
	if err != nil {
		return "", err
	}

	if doc.Mods == nil {
		doc.Mods = make(map[string]ModRecord)
	}
	doc.Mods[id] = ModRecord{ScannerID: scannerID}
	return id, nil
}

// Returns the persistent identity currently mapped to scannerID, if any.
func (doc Document) FindModByScannerID(scannerID string) (string, bool) {
	for id, record := range doc.Mods {
		if record.ScannerID == scannerID {
			return id, true
		}
	}
	return "", false
}

// Re-points the mod record tracking previousScannerID at scannerID instead,
// so a mutation result's PreviousID/ID pair (from rename, priority change, or
// move) keeps existing metadata attached to the same mod rather than
// orphaning it under its old scanner ID. Reports whether a record was found.
func (doc *Document) ReconcileMod(previousScannerID, scannerID string) bool {
	id, ok := doc.FindModByScannerID(previousScannerID)
	if !ok {
		return false
	}

	record := doc.Mods[id]
	record.ScannerID = scannerID
	doc.Mods[id] = record
	return true
}

// Reports mod records whose ScannerID does not appear in liveScannerIDs, the
// entry IDs from a fresh scan. An orphaned record is not removed here:
// callers surface it instead of discarding it, since the mod may reappear,
// for example after reselecting a folder.
func (doc Document) OrphanedMods(liveScannerIDs map[string]struct{}) map[string]ModRecord {
	orphaned := make(map[string]ModRecord)
	for id, record := range doc.Mods {
		if _, present := liveScannerIDs[record.ScannerID]; !present {
			orphaned[id] = record
		}
	}
	return orphaned
}

// Generates an opaque identity, prefixed by kind, independent of any
// filesystem path.
func newID(prefix string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate identity: %w", err)
	}
	return prefix + hex.EncodeToString(raw), nil
}
