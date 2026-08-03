package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	groupKeySeparator          = "\x00"
	prioritySuffix             = "_P"
	prioritySeparator          = "_"
	minimumTrailingNines       = 7
	trailingNinePriorityOffset = minimumTrailingNines - 1
)

// State describes whether a primary file is enabled or disabled.
type State string

const (
	StateEnabled  State = "enabled"
	StateDisabled State = "disabled"
)

// DisabledFormat identifies the disabled filename convention, when present.
type DisabledFormat string

const (
	DisabledFormatNone     DisabledFormat = ""
	DisabledFormatCrateoff DisabledFormat = ".pak_crateoff"
	DisabledFormatBento    DisabledFormat = ".bak_bento"
	DisabledFormatLegacy   DisabledFormat = ".pak_disabled"
)

// EntryKind describes whether an entry has a primary file or only sidecars.
type EntryKind string

const (
	EntryMod             EntryKind = "mod"
	EntryOrphanedSidecar EntryKind = "orphaned_sidecar"
)

// BundleFormat describes the recognized bundle format for a primary-backed entry.
type BundleFormat string

const (
	BundleFormatNone    BundleFormat = ""
	BundleFormatClassic BundleFormat = "classic"
	BundleFormatIoStore BundleFormat = "iostore"
)

// PriorityKind records how a filename's priority was interpreted.
type PriorityKind string

const (
	PriorityNone         PriorityKind = "none"
	PriorityLeadingBang  PriorityKind = "leading_bang"
	PriorityTrailingNine PriorityKind = "trailing_nines"
	PriorityUnrecognized PriorityKind = "unrecognized"
)

// Priority contains the parsed priority and the evidence used to derive it.
type Priority struct {
	Value         int          `json:"value"`
	Kind          PriorityKind `json:"kind"`
	Raw           string       `json:"raw"`
	TrailingNines int          `json:"trailingNines"`
}

// Sidecars records recognized same-stem sidecar paths, relative to the root.
type Sidecars struct {
	UTOC string `json:"utoc,omitempty"`
	UCAS string `json:"ucas,omitempty"`
}

// IssueCode identifies an unusual or incomplete discovered condition.
type IssueCode string

const (
	IssueMissingUTOC      IssueCode = "missing-utoc"
	IssueMissingUCAS      IssueCode = "missing-ucas"
	IssueAmbiguousPrimary IssueCode = "ambiguous-primary"
	IssueOrphanedSidecar  IssueCode = "orphaned-sidecar"
)

// Issue is a read-only explanation attached to a discovered entry.
type Issue struct {
	Code    IssueCode `json:"code"`
	Message string    `json:"message"`
}

// Entry describes one primary file or one orphaned sidecar group.
type Entry struct {
	PrimaryPath    string         `json:"primaryPath,omitempty"`
	RelativeFolder string         `json:"relativeFolder"`
	DisplayName    string         `json:"displayName"`
	State          State          `json:"state"`
	DisabledFormat DisabledFormat `json:"disabledFormat,omitempty"`
	Kind           EntryKind      `json:"kind"`
	BundleFormat   BundleFormat   `json:"bundleFormat,omitempty"`
	Sidecars       Sidecars       `json:"sidecars"`
	Priority       Priority       `json:"priority"`
	Issues         []Issue        `json:"issues,omitempty"`
}

// Library is the deterministic result of scanning one mod root.
type Library struct {
	Root    string  `json:"root"`
	Entries []Entry `json:"entries"`
}

// Scan recursively discovers supported primaries and sidecars under root.
// It never writes to the filesystem.
func Scan(root string) (Library, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Library{}, fmt.Errorf("stat mod root: %w", err)
	}

	if !info.IsDir() {
		return Library{}, fmt.Errorf("mod root is not a directory: %s", root)
	}

	files := make(map[string][]fileRecord)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		kind, ok := classifyExtension(entry.Name())
		if !ok {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		relative = filepath.ToSlash(relative)
		baseName := filepath.Base(relative)
		stem := baseName[:len(baseName)-len(kind)]
		groupKey := fileGroupKey(relative, stem)
		files[groupKey] = append(files[groupKey], fileRecord{
			path: relative,
			stem: stem,
			kind: kind,
		})
		return nil
	})
	if err != nil {
		return Library{}, fmt.Errorf("scan mod root: %w", err)
	}

	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := Library{Root: root}
	for _, key := range keys {
		records := files[key]
		sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })
		primaries, sidecars := splitRecords(records)

		if len(primaries) == 0 {
			result.Entries = append(result.Entries, orphanEntry(records[0].stem, records[0].path, sidecars))
			continue
		}
		ambiguous := len(primaries) > 1
		for _, primary := range primaries {
			entry := primaryEntry(primary, sidecars, ambiguous)
			result.Entries = append(result.Entries, entry)
		}
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		return entrySortKey(result.Entries[i]) < entrySortKey(result.Entries[j])
	})
	return result, nil
}

type extension string

const (
	extensionPrimaryPak extension = ".pak"
	extensionCrateoff   extension = ".pak_crateoff"
	extensionBento      extension = ".bak_bento"
	extensionLegacy     extension = ".pak_disabled"
	extensionUTOC       extension = ".utoc"
	extensionUCAS       extension = ".ucas"
)

type fileRecord struct {
	path string
	stem string
	kind extension
}

// fileGroupKey groups case-insensitively while retaining original path casing for display.
func fileGroupKey(path, stem string) string {
	return strings.ToLower(relativeFolder(path)) + groupKeySeparator + strings.ToLower(stem)
}

func relativeFolder(path string) string {
	folder := filepath.ToSlash(filepath.Dir(path))
	if folder == "." {
		return ""
	}

	return folder
}

// splitRecords shares sidecars with every primary in an ambiguous same-stem group.
func splitRecords(records []fileRecord) ([]fileRecord, Sidecars) {
	var primaries []fileRecord
	var sidecars Sidecars

	for _, record := range records {
		switch record.kind {
		case extensionPrimaryPak, extensionCrateoff, extensionBento, extensionLegacy:
			primaries = append(primaries, record)
		case extensionUTOC:
			sidecars.UTOC = record.path
		case extensionUCAS:
			sidecars.UCAS = record.path
		}
	}

	return primaries, sidecars
}

func entrySortKey(entry Entry) string {
	return entry.RelativeFolder + groupKeySeparator + entry.PrimaryPath + groupKeySeparator + entry.DisplayName
}

func classifyExtension(name string) (extension, bool) {
	lower := strings.ToLower(name)
	for _, candidate := range []extension{extensionCrateoff, extensionLegacy, extensionBento, extensionPrimaryPak, extensionUTOC, extensionUCAS} {
		if strings.HasSuffix(lower, string(candidate)) {
			return candidate, true
		}
	}
	return "", false
}

func primaryEntry(primary fileRecord, sidecars Sidecars, ambiguous bool) Entry {
	folder := relativeFolder(primary.path)
	state := StateEnabled
	disabledFormat := DisabledFormatNone
	switch primary.kind {
	case extensionCrateoff:
		state, disabledFormat = StateDisabled, DisabledFormatCrateoff
	case extensionBento:
		state, disabledFormat = StateDisabled, DisabledFormatBento
	case extensionLegacy:
		state, disabledFormat = StateDisabled, DisabledFormatLegacy
	}

	format := BundleFormatClassic
	var issues []Issue
	if sidecars.UTOC != "" || sidecars.UCAS != "" {
		format = BundleFormatIoStore
		if sidecars.UTOC == "" {
			issues = append(issues, Issue{Code: IssueMissingUTOC, Message: "IoStore bundle is missing its .utoc sidecar"})
		}
		if sidecars.UCAS == "" {
			issues = append(issues, Issue{Code: IssueMissingUCAS, Message: "IoStore bundle is missing its .ucas sidecar"})
		}
	}

	if ambiguous {
		issues = append(issues, Issue{Code: IssueAmbiguousPrimary, Message: "Multiple supported primaries share this folder and filename stem"})
	}

	return Entry{
		PrimaryPath:    primary.path,
		RelativeFolder: folder,
		DisplayName:    cleanDisplayName(primary.stem),
		State:          state,
		DisabledFormat: disabledFormat,
		Kind:           EntryMod,
		BundleFormat:   format,
		Sidecars:       sidecars,
		Priority:       parsePriority(primary.stem),
		Issues:         issues,
	}
}

func orphanEntry(stem, path string, sidecars Sidecars) Entry {
	folder := relativeFolder(path)

	return Entry{
		RelativeFolder: folder,
		DisplayName:    cleanDisplayName(stem),
		Kind:           EntryOrphanedSidecar,
		BundleFormat:   BundleFormatNone,
		Sidecars:       sidecars,
		Priority:       parsePriority(stem),
		Issues:         []Issue{{Code: IssueOrphanedSidecar, Message: "Sidecar has no supported primary file"}},
	}
}

func cleanDisplayName(stem string) string {
	name := strings.TrimPrefix(stem, "!")
	if suffixStart := trailingNinesSuffixStart(name); suffixStart >= 0 {
		name = name[:suffixStart]
	}
	return strings.TrimSuffix(name, "_")
}

// parsePriority applies the explicit leading-bang convention before trailing nines.
func parsePriority(stem string) Priority {
	raw := stem
	if strings.HasPrefix(stem, "!") {
		return Priority{Kind: PriorityLeadingBang, Raw: raw, TrailingNines: trailingNineCount(stem)}
	}

	if suffixStart := trailingNinesSuffixStart(stem); suffixStart >= 0 {
		nines := len(stem[suffixStart+1 : len(stem)-2])
		return Priority{Value: nines - trailingNinePriorityOffset, Kind: PriorityTrailingNine, Raw: raw, TrailingNines: nines}
	}

	if strings.HasSuffix(stem, prioritySuffix) {
		return Priority{Kind: PriorityUnrecognized, Raw: raw}
	}
	return Priority{Kind: PriorityNone, Raw: raw}
}

// trailingNinesSuffixStart returns the separator before a recognized trailing-nines priority.
func trailingNinesSuffixStart(stem string) int {
	if !strings.HasSuffix(stem, prioritySuffix) {
		return -1
	}
	separator := strings.LastIndex(stem[:len(stem)-len(prioritySuffix)], prioritySeparator)
	if separator < 0 {
		return -1
	}
	digits := stem[separator+1 : len(stem)-len(prioritySuffix)]
	if len(digits) < minimumTrailingNines {
		return -1
	}
	for _, digit := range digits {
		if digit != '9' {
			return -1
		}
	}
	return separator
}

func trailingNineCount(stem string) int {
	end := strings.LastIndex(stem, prioritySuffix)
	if end < 0 {
		return 0
	}
	start := strings.LastIndex(stem[:end], prioritySeparator) + 1
	count := 0
	for _, digit := range stem[start:end] {
		if digit != '9' {
			return 0
		}
		count++
	}
	return count
}
