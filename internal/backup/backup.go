// Package backup zips a mod library and Cratebug's metadata into a portable
// backup file. It is read-only against the library: the only filesystem
// write is the destination zip itself, which must live outside the mod root.
package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

const (
	// Name of Cratebug's metadata file where it is stored inside a backup.
	metadataEntryName = "metadata.json"

	// Permissions for the destination zip, matching the metadata store's
	// private-file convention.
	destinationFilePermissions = 0600
)

// Tallies the scanned library for the backup report. The three categories
// are disjoint and sum to Mods.
type Counts struct {
	Mods    int `json:"mods"`
	IoStore int `json:"iostore"`
	Classic int `json:"classic"`
	Invalid int `json:"invalid"`
}

// Backup advancement, emitted as one event per written file.
type Progress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentFile string `json:"currentFile"`
}

// The outcome of a backup: what was archived and where it went. A
// cancelled save dialog reports Cancelled instead of an error so the
// frontend can stay silent.
type Result struct {
	Counts           Counts `json:"counts"`
	DestinationPath  string `json:"destinationPath"`
	MetadataIncluded bool   `json:"metadataIncluded"`
	Cancelled        bool   `json:"cancelled"`
}

// Tallies scan entries into disjoint report categories. An entry is invalid
// when the scanner flagged it or it is not a primary-backed mod; valid mods
// split by bundle format.
func CountsLibrary(entries []discovery.Entry) Counts {
	var counts Counts
	for _, entry := range entries {
		if entry.Kind != discovery.EntryMod || len(entry.Issues) > 0 {
			counts.Invalid++
			continue
		}
		if entry.BundleFormat == discovery.BundleFormatIoStore {
			counts.IoStore++
			continue
		}
		counts.Classic++
	}
	counts.Mods = counts.IoStore + counts.Classic + counts.Invalid
	return counts
}

// Archives the mod-root tree plus the metadata file at metadataPath into
// destPath and returns the library counts. The scan is read-only; destPath
// must not sit inside the mod root. A missing metadata file is not an error:
// the tree is backed up without it. Cancellation or any failure removes the
// partial destination so it is never presented as a completed backup.
func Create(ctx context.Context, modRoot, metadataPath, destPath string, onProgress func(Progress)) (Result, error) {
	library, err := discovery.Scan(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scan mod library before backup: %w", err)
	}
	counts := CountsLibrary(library.Entries)

	root, err := filepath.Abs(modRoot)
	if err != nil {
		return Result{}, fmt.Errorf("resolve mod library path: %w", err)
	}
	dest, err := filepath.Abs(destPath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve backup destination: %w", err)
	}
	// A backup written inside the library would pollute later scans and
	// could be swept into the next backup, so the destination must live
	// outside the mod root. A Rel failure (such as a different volume)
	// means the destination cannot be inside the root.
	if rel, err := filepath.Rel(root, dest); err == nil && filepath.IsLocal(rel) {
		return Result{}, fmt.Errorf("backup destination must live outside the mod library")
	}

	files, err := collectFiles(root)
	if err != nil {
		return Result{}, err
	}

	metadataBytes, metadataIncluded, err := readMetadata(metadataPath)
	if err != nil {
		return Result{}, err
	}
	if metadataIncluded {
		for _, file := range files {
			if file.rel == metadataEntryName {
				return Result{}, fmt.Errorf("mod library contains a top-level %q that collides with the backup metadata entry", metadataEntryName)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("backup cancelled before writing: %w", err)
	}

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, destinationFilePermissions)
	if err != nil {
		return Result{}, fmt.Errorf("create backup destination: %w", err)
	}
	writeErr := writeZip(ctx, out, root, files, metadataBytes, metadataIncluded, onProgress)
	closeErr := out.Close()
	if writeErr != nil {
		_ = os.Remove(dest)
		return Result{}, writeErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return Result{}, fmt.Errorf("finish backup destination: %w", closeErr)
	}

	return Result{Counts: counts, DestinationPath: dest, MetadataIncluded: metadataIncluded}, nil
}

// One library file collected for the zip, with its slash-separated path
// relative to the mod root.
type backupFile struct {
	rel string
}

// Lists every file beneath root in deterministic order. Empty directories
// are preserved through explicit zip directory entries at write time, so
// only files are collected here.
func collectFiles(root string) ([]backupFile, error) {
	var files []backupFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, backupFile{rel: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list mod library files: %w", err)
	}
	return files, nil
}

// Returns the metadata file bytes, or reports it absent. A missing file is
// fine; an unreadable one is an error rather than a silent metadata-free
// backup.
func readMetadata(metadataPath string) ([]byte, bool, error) {
	if metadataPath == "" {
		return nil, false, nil
	}
	contents, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read metadata file: %w", err)
	}
	return contents, true, nil
}

// Streams the collected files plus the metadata entry into out, emitting one
// progress event per file. Context cancellation aborts the write; the caller
// removes the partial destination.
func writeZip(ctx context.Context, out io.Writer, root string, files []backupFile, metadataBytes []byte, metadataIncluded bool, onProgress func(Progress)) error {
	writer := zip.NewWriter(out)
	total := len(files)
	if metadataIncluded {
		total++
	}
	written := 0
	report := func(rel string) {
		written++
		if onProgress != nil {
			onProgress(Progress{Current: written, Total: total, CurrentFile: rel})
		}
	}

	seenDirs := make(map[string]struct{})
	addDirEntries := func(rel string) error {
		dir := rel
		if idx := strings.LastIndexByte(dir, '/'); idx >= 0 {
			dir = dir[:idx]
		} else {
			return nil
		}
		var chain []string
		for current := dir; ; {
			if _, seen := seenDirs[current]; seen {
				break
			}
			seenDirs[current] = struct{}{}
			chain = append(chain, current+"/")
			idx := strings.LastIndexByte(current, '/')
			if idx < 0 {
				break
			}
			current = current[:idx]
		}
		for i := len(chain) - 1; i >= 0; i-- {
			if _, err := writer.Create(chain[i]); err != nil {
				return fmt.Errorf("write backup directory entry %q: %w", chain[i], err)
			}
		}
		return nil
	}

	addFile := func(rel string, contents []byte) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("backup cancelled: %w", err)
		}
		if err := addDirEntries(rel); err != nil {
			return err
		}
		entry, err := writer.Create(rel)
		if err != nil {
			return fmt.Errorf("write backup entry %q: %w", rel, err)
		}
		if _, err := entry.Write(contents); err != nil {
			return fmt.Errorf("write backup entry %q: %w", rel, err)
		}
		report(rel)
		return nil
	}

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			_ = writer.Close()
			return fmt.Errorf("backup cancelled: %w", err)
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.rel)))
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("read mod library file %q: %w", file.rel, err)
		}
		if err := addFile(file.rel, contents); err != nil {
			_ = writer.Close()
			return err
		}
	}

	if metadataIncluded {
		if err := addFile(metadataEntryName, metadataBytes); err != nil {
			_ = writer.Close()
			return err
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish backup archive: %w", err)
	}
	return nil
}
