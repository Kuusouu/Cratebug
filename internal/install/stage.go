package install

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

const (
	// Number of random bytes used to generate a unique staging session ID.
	sessionIDEntropyBytes = 16

	// Restricted permissions for the temporary staging workspace directory.
	privateStagingDirPermissions = 0700

	// Permissions for item subdirectories within staging.
	defaultStagingDirPermissions = 0755

	// Permissions for copied files in staging.
	defaultStagingFilePermissions = 0644

	// Standard Marvel Rivals patch suffix stripped from display names for clean preview.
	stagingPatchSuffix = "_P"

	// Reported when a companion sidecar or primary fails to copy during staging, so the
	// resulting bundle is not installed as if it were complete.
	issueCompanionCopyFailed discovery.IssueCode = "companion-copy-failed"
)

// Progress reports the current status of a staging or installation operation.
type Progress struct {
	Phase   string  `json:"phase"`   // e.g. "extracting", "staging", "applying"
	Current int     `json:"current"` // current item index (1-based)
	Total   int     `json:"total"`   // total items
	Message string  `json:"message"` // human-readable status
	Percent float64 `json:"percent"` // 0.0 to 100.0
}

// StagedMod describes one discovered Unreal mod bundle within a staged session.
type StagedMod struct {
	ID                  string                 `json:"id"`
	RelativePrimaryPath string                 `json:"relativePrimaryPath"`
	SourcePath          string                 `json:"sourcePath"`
	Sidecars            discovery.Sidecars     `json:"sidecars"`
	BundleFormat        discovery.BundleFormat `json:"bundleFormat"`
	DisplayName         string                 `json:"displayName"`
	Stem                string                 `json:"stem"`
	TotalSizeBytes      int64                  `json:"totalSizeBytes"`
	AllFiles            []string               `json:"allFiles"`
	Issues              []discovery.Issue      `json:"issues,omitempty"`
}

// StagedSession holds the temporary workspace where mod files are unpacked and inspected.
type StagedSession struct {
	ID          string      `json:"id"`
	Dir         string      `json:"dir"`
	SourceFiles []string    `json:"sourceFiles"`
	Mods        []StagedMod `json:"mods"`
}

// SessionManager coordinates active staging sessions across Wails calls.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*StagedSession
}

// Creates a new thread-safe session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*StagedSession),
	}
}

// Generates a cryptographically random session identifier.
func generateSessionID() (string, error) {
	bytes := make([]byte, sessionIDEntropyBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// Creates a new staging directory and session.
func (sm *SessionManager) CreateSession(sourceFiles []string) (*StagedSession, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	stagingDir := filepath.Join(os.TempDir(), "cratebug-staging-"+id)
	if err := os.MkdirAll(stagingDir, privateStagingDirPermissions); err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}

	session := &StagedSession{
		ID:          id,
		Dir:         stagingDir,
		SourceFiles: sourceFiles,
	}

	sm.mu.Lock()
	sm.sessions[id] = session
	sm.mu.Unlock()

	return session, nil
}

// Returns the staged session with the specified ID, or nil if not found.
func (sm *SessionManager) GetSession(id string) *StagedSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[id]
}

// Cleans up the staging folder on disk and removes the session from the manager.
func (sm *SessionManager) RemoveSession(id string) error {
	sm.mu.Lock()
	session, exists := sm.sessions[id]
	if exists {
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()

	if !exists || session == nil {
		return nil
	}

	return session.Cleanup()
}

// Cleans up all active staging workspaces on disk and empties the session registry.
func (sm *SessionManager) CleanupAll() error {
	sm.mu.Lock()
	sessions := make([]*StagedSession, 0, len(sm.sessions))
	for id, session := range sm.sessions {
		sessions = append(sessions, session)
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()

	var errs []string
	for _, session := range sessions {
		if session != nil {
			if err := session.Cleanup(); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup staging sessions: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Cleans up all files on disk associated with the staging session.
func (s *StagedSession) Cleanup() error {
	if s.Dir == "" {
		return nil
	}
	return os.RemoveAll(s.Dir)
}

// StageFiles unpacks archives or copies loose files into the session directory,
// then discovers and classifies all mod bundles found within.
func (s *StagedSession) StageFiles(ctx context.Context, onProgress func(Progress)) error {
	total := len(s.SourceFiles)
	if total == 0 {
		return fmt.Errorf("no files provided for staging")
	}

	// Records companion files that failed to copy during auto-discovery, keyed by the
	// lowercased stem of the loose file that triggered discovery. discoverStagedMods uses
	// this to flag bundles that look complete on disk only because a copy silently failed.
	companionCopyFailures := make(map[string][]string)

	for i, srcPath := range s.SourceFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fileName := filepath.Base(srcPath)
		if onProgress != nil {
			onProgress(Progress{
				Phase:   "staging",
				Current: i + 1,
				Total:   total,
				Message: fmt.Sprintf("Processing %s (%d of %d)", fileName, i+1, total),
				Percent: float64(i) / float64(total) * 100.0,
			})
		}

		if IsArchiveFile(srcPath) {
			itemDir := filepath.Join(s.Dir, fmt.Sprintf("item_%d", i))
			if err := os.MkdirAll(itemDir, defaultStagingDirPermissions); err != nil {
				return fmt.Errorf("create staging item directory %q: %w", itemDir, err)
			}

			if err := ExtractArchive(ctx, srcPath, itemDir); err != nil {
				return fmt.Errorf("extract %q: %w", fileName, err)
			}
		} else {
			looseDir := filepath.Join(s.Dir, "loose")
			if err := os.MkdirAll(looseDir, defaultStagingDirPermissions); err != nil {
				return fmt.Errorf("create staging loose directory %q: %w", looseDir, err)
			}

			// Identically named files from different source directories will
			// overwrite each other here, matching normal filesystem semantics.
			destFile := filepath.Join(looseDir, fileName)
			if err := copyRegularFile(srcPath, destFile); err != nil {
				return fmt.Errorf("copy %q to staging: %w", fileName, err)
			}

			// Automatically discover and pull in matching companion sidecars (.utoc, .ucas)
			// or primary (.pak) from the same source directory so selecting only the .pak
			// completes the full IoStore bundle.
			srcDir := filepath.Dir(srcPath)
			stem := extractFileStem(fileName)
			if dirEntries, err := os.ReadDir(srcDir); err == nil {
				for _, entry := range dirEntries {
					if entry.IsDir() {
						continue
					}
					entryName := entry.Name()
					if strings.EqualFold(entryName, fileName) {
						continue
					}
					if strings.EqualFold(extractFileStem(entryName), stem) {
						if hasBundleExtension(entryName) {
							companionSrc := filepath.Join(srcDir, entryName)
							companionDst := filepath.Join(looseDir, entryName)
							if err := copyRegularFile(companionSrc, companionDst); err != nil {
								// Remove any partial write so a truncated copy doesn't
								// masquerade as a genuinely present sidecar.
								_ = os.Remove(companionDst)
								key := strings.ToLower(stem)
								companionCopyFailures[key] = append(companionCopyFailures[key], entryName)
							}
						}
					}
				}
			}
		}
	}

	if onProgress != nil {
		onProgress(Progress{
			Phase:   "discovering",
			Current: total,
			Total:   total,
			Message: "Discovering mod bundles...",
			Percent: 100.0,
		})
	}

	mods, err := discoverStagedMods(s.Dir, s.SourceFiles, companionCopyFailures)
	if err != nil {
		return fmt.Errorf("discover staged mods: %w", err)
	}

	if len(mods) == 0 {
		return fmt.Errorf("no valid Marvel Rivals mod files (.pak) found in selected files")
	}

	s.Mods = mods
	return nil
}

// Scans the staging workspace and builds StagedMod representations. companionCopyFailures
// flags bundles whose companion files failed to copy during staging, keyed by lowercased stem.
func discoverStagedMods(stagingRoot string, sourceFiles []string, companionCopyFailures map[string][]string) ([]StagedMod, error) {
	lib, err := discovery.Scan(stagingRoot)
	if err != nil {
		return nil, fmt.Errorf("scan staging directory: %w", err)
	}

	var mods []StagedMod
	for i, entry := range lib.Entries {
		if entry.Kind != discovery.EntryMod || entry.PrimaryPath == "" {
			continue
		}

		var allFiles []string
		var totalBytes int64

		// Collect primary file info
		primaryAbs := filepath.Join(stagingRoot, filepath.FromSlash(entry.PrimaryPath))
		if info, err := os.Stat(primaryAbs); err == nil {
			totalBytes += info.Size()
			allFiles = append(allFiles, entry.PrimaryPath)
		}

		// Collect sidecars info
		if entry.Sidecars.UTOC != "" {
			utocAbs := filepath.Join(stagingRoot, filepath.FromSlash(entry.Sidecars.UTOC))
			if info, err := os.Stat(utocAbs); err == nil {
				totalBytes += info.Size()
				allFiles = append(allFiles, entry.Sidecars.UTOC)
			}
		}
		if entry.Sidecars.UCAS != "" {
			ucasAbs := filepath.Join(stagingRoot, filepath.FromSlash(entry.Sidecars.UCAS))
			if info, err := os.Stat(ucasAbs); err == nil {
				totalBytes += info.Size()
				allFiles = append(allFiles, entry.Sidecars.UCAS)
			}
		}

		primaryBase := filepath.Base(entry.PrimaryPath)
		stem := extractFileStem(primaryBase)

		issues := append([]discovery.Issue{}, entry.Issues...)
		for _, failedFile := range companionCopyFailures[strings.ToLower(stem)] {
			issues = append(issues, discovery.Issue{
				Code:    issueCompanionCopyFailed,
				Message: fmt.Sprintf("%s failed to copy during staging; the bundle may be incomplete", failedFile),
			})
		}

		modID := fmt.Sprintf("staged-mod-%d", i)
		mods = append(mods, StagedMod{
			ID:                  modID,
			RelativePrimaryPath: entry.PrimaryPath,
			SourcePath:          determineSourcePath(entry.PrimaryPath, sourceFiles),
			Sidecars:            entry.Sidecars,
			BundleFormat:        entry.BundleFormat,
			DisplayName:         cleanInstallDisplayName(entry.DisplayName),
			Stem:                stem,
			TotalSizeBytes:      totalBytes,
			AllFiles:            allFiles,
			Issues:              issues,
		})
	}

	sort.Slice(mods, func(i, j int) bool {
		return mods[i].DisplayName < mods[j].DisplayName
	})

	return mods, nil
}

// Identifies which selected source input produced a staged path, so origin-formatting
// helpers don't each re-derive the same loose-file/archive-item resolution.
type stagedOrigin struct {
	fromArchive bool // true for an "item_N/..." path staged out of an archive

	// Set when fromArchive is false (a directly-selected "loose/..." file).
	looseSourcePath string // best-guess absolute path of the selected or companion source file
	looseFileName   string // staged file's base name

	// Set when fromArchive is true.
	archiveSourcePath string
	archiveInnerPath  string
	multiSource       bool // true when more than one source file was selected
}

// Resolves which sourceFiles entry a staged primaryPath came from, if any.
func resolveStagedOrigin(primaryPath string, sourceFiles []string) (stagedOrigin, bool) {
	cleanRel := filepath.ToSlash(primaryPath)

	if strings.HasPrefix(cleanRel, "loose/") {
		fileName := strings.TrimPrefix(cleanRel, "loose/")
		for _, src := range sourceFiles {
			if strings.EqualFold(filepath.Base(src), fileName) {
				return stagedOrigin{looseSourcePath: src, looseFileName: fileName}, true
			}
			companion := filepath.Join(filepath.Dir(src), fileName)
			if _, err := os.Stat(companion); err == nil {
				return stagedOrigin{looseSourcePath: companion, looseFileName: fileName}, true
			}
		}
		return stagedOrigin{looseFileName: fileName}, false
	}

	if strings.HasPrefix(cleanRel, "item_") {
		parts := strings.SplitN(cleanRel, "/", 2)
		if len(parts) == 2 {
			var index int
			if _, err := fmt.Sscanf(parts[0], "item_%d", &index); err == nil && index >= 0 && index < len(sourceFiles) {
				return stagedOrigin{
					fromArchive:       true,
					archiveSourcePath: sourceFiles[index],
					archiveInnerPath:  parts[1],
					multiSource:       len(sourceFiles) > 1,
				}, true
			}
		}
	}

	return stagedOrigin{}, false
}

// Determines the readable origin path for a staged mod candidate.
func determineSourcePath(primaryPath string, sourceFiles []string) string {
	origin, ok := resolveStagedOrigin(primaryPath, sourceFiles)
	if !ok {
		if origin.looseFileName != "" {
			return origin.looseFileName
		}
		return primaryPath
	}
	if origin.fromArchive {
		return fmt.Sprintf("%s > %s", origin.archiveSourcePath, origin.archiveInnerPath)
	}
	return origin.looseSourcePath
}

// Determines the concise readable display path for a file in the preview UI.
func determineDisplayPath(primaryPath string, sourceFiles []string) string {
	origin, ok := resolveStagedOrigin(primaryPath, sourceFiles)
	if !ok {
		if origin.looseFileName != "" {
			return origin.looseFileName
		}
		return primaryPath
	}
	if origin.fromArchive {
		if origin.multiSource {
			return fmt.Sprintf("%s > %s", filepath.Base(origin.archiveSourcePath), origin.archiveInnerPath)
		}
		return origin.archiveInnerPath
	}
	parent := filepath.Base(filepath.Dir(origin.looseSourcePath))
	if parent != "" && parent != "." && parent != "/" && parent != "\\" {
		return filepath.ToSlash(filepath.Join(parent, origin.looseFileName))
	}
	return origin.looseFileName
}

// Extracts the filename stem excluding extension and disabled suffixes.
func extractFileStem(fileName string) string {
	lower := strings.ToLower(fileName)
	for _, ext := range []string{".pak_crateoff", ".bak_bento", ".pak_disabled", ".pak"} {
		if strings.HasSuffix(lower, ext) {
			return fileName[:len(fileName)-len(ext)]
		}
	}
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

// Cleans common priority and patch suffixes from display names for preview.
func cleanInstallDisplayName(name string) string {
	cleaned := strings.TrimPrefix(name, "!")
	if strings.HasSuffix(strings.ToUpper(cleaned), stagingPatchSuffix) {
		cleaned = cleaned[:len(cleaned)-len(stagingPatchSuffix)]
	}
	return strings.TrimSuffix(cleaned, "_")
}

// Safely copies one regular file from src to dst.
func copyRegularFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", src, err)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", src, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), defaultStagingDirPermissions); err != nil {
		return fmt.Errorf("create destination directory for %q: %w", dst, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultStagingFilePermissions)
	if err != nil {
		return fmt.Errorf("create destination file %q: %w", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return fmt.Errorf("copy data from %q to %q: %w", src, dst, err)
	}

	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("flush destination file %q: %w", dst, err)
	}

	return nil
}
