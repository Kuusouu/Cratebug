package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/update"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The GitHub repository release.yml publishes to and CheckForUpdate reads from.
const (
	updateRepoOwner = "Kuusouu"
	updateRepoName  = "Cratebug"

	// Deliberately distinct from internal/update's own applyStagingDirName:
	// writeApplyScript clears its staging directory with os.RemoveAll before
	// writing the helper script, which would delete the downloaded installer
	// if the two shared a directory.
	updateDownloadDirName = "cratebug-update-download"
)

// Update checks and installation, bound to the frontend.
type UpdateService struct {
	*core
}

// Reports whether a newer Cratebug release is published, and its details when it is.
type UpdateCheckResult struct {
	Available bool           `json:"available"`
	Release   update.Release `json:"release,omitempty"`
}

// Reports download progress for an in-progress update download, emitted on
// the "update:downloadProgress" event as it downloads (matching
// PrepareInstall's "install:progress" event for the same purpose).
type UpdateDownloadProgress struct {
	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
}

func newUpdateService(c *core) *UpdateService {
	return &UpdateService{core: c}
}

// GetAppVersion returns the running build's CalVer release tag, or "dev" for
// a local or CI build, for display in Settings.
func (s *UpdateService) GetAppVersion() string {
	return AppVersion
}

// CheckForUpdate compares the running build against the latest published
// GitHub release. A local or CI build (AppVersion is "dev", not a real
// release tag) never reports one available, since there is nothing valid to
// compare against.
func (s *UpdateService) CheckForUpdate() (UpdateCheckResult, error) {
	current, err := update.ParseVersion(AppVersion)
	if err != nil {
		return UpdateCheckResult{}, nil
	}

	release, err := s.updateClient().Latest(s.runtime.context())
	if err != nil {
		if errors.Is(err, update.ErrNoRelease) {
			return UpdateCheckResult{}, nil
		}
		return UpdateCheckResult{}, err
	}

	if !release.Version.IsNewer(current) {
		return UpdateCheckResult{}, nil
	}
	return UpdateCheckResult{Available: true, Release: release}, nil
}

// CheckWhatsNew reports the changelog for the running build's own release,
// once, the first time it runs after that build changed -- the "what's new"
// notice shown right after an update relaunches Cratebug, not a check for a
// further update. Persists the running version as seen immediately so a
// crash before the frontend renders the notice can only lose it, never
// repeat it on every subsequent launch.
func (s *UpdateService) CheckWhatsNew() (UpdateCheckResult, error) {
	current, err := update.ParseVersion(AppVersion)
	if err != nil {
		return UpdateCheckResult{}, nil
	}

	if s.meta.read().Settings.LastSeenVersion == current.Tag {
		return UpdateCheckResult{}, nil
	}

	markSeen := func(doc *metadata.Document) error {
		doc.SetLastSeenVersion(current.Tag)
		return nil
	}

	// The release lookup runs outside the metadata lock so a slow network
	// call cannot hold up other settings writes.
	release, err := s.updateClient().Tag(s.runtime.context(), current.Tag)
	if err != nil {
		// Best-effort: a missing or unreachable release for the running tag
		// (offline, or a build that was never actually published) should not
		// block startup or repeat every launch, so it's still marked seen.
		_ = s.meta.update(markSeen)
		return UpdateCheckResult{}, nil
	}

	if saveErr := s.meta.update(markSeen); saveErr != nil {
		return UpdateCheckResult{}, fmt.Errorf("record the shown changelog version: %w", saveErr)
	}
	return UpdateCheckResult{Available: true, Release: release}, nil
}

// DownloadUpdate downloads release's installer, reporting progress through
// the "update:downloadProgress" event. Returns the downloaded installer's
// local path for a subsequent ApplyUpdate call.
func (s *UpdateService) DownloadUpdate(release update.Release) (string, error) {
	destDir := filepath.Join(os.TempDir(), updateDownloadDirName)
	onProgress := func(downloaded, total int64) {
		s.runtime.emit("update:downloadProgress", UpdateDownloadProgress{
			Downloaded: downloaded,
			Total:      total,
		})
	}
	return s.updateClient().Download(s.runtime.context(), release.Asset, destDir, onProgress)
}

// ApplyUpdate launches the detached helper that installs the update
// downloaded to installerPath and relaunches Cratebug, then quits the
// running instance so the helper can replace it. A nil error means Cratebug
// is about to exit, not that the update finished applying -- that happens in
// the detached helper after this process is gone.
func (s *UpdateService) ApplyUpdate(installerPath string) error {
	// Wails exposes every bound method to the frontend with no lower trust
	// boundary, so this can't rely on the frontend only ever passing back
	// exactly what DownloadUpdate gave it. Unlike the mutation methods
	// (RenameMod, DeleteMod, ...), which are inherently constrained to paths
	// already inside the user's mod library, a bare installerPath string has
	// no such constraint -- confirm it actually points at something
	// DownloadUpdate could have produced before spawning it as a process.
	expectedDir := filepath.Join(os.TempDir(), updateDownloadDirName)
	if !strings.EqualFold(filepath.Ext(installerPath), ".exe") || !pathWithinDir(expectedDir, installerPath) {
		return fmt.Errorf("refusing to apply update from an unexpected path: %q", installerPath)
	}

	// Checked here rather than in the helper so the failure surfaces while
	// the app can still report it: after this call succeeds the app quits,
	// and a helper whose installer vanished (antivirus quarantine, a temp
	// cleaner) would otherwise close Cratebug silently without relaunching.
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("downloaded installer %q is not available: %w", installerPath, err)
	}

	if err := update.ApplyUpdate(installerPath); err != nil {
		return err
	}
	if ctx := s.runtime.current(); ctx != nil {
		wailsRuntime.Quit(ctx)
	}
	return nil
}

// Reports whether path resolves to a location inside dir, rejecting any
// ".." traversal component.
func pathWithinDir(dir, path string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && filepath.IsLocal(relative)
}

func (s *UpdateService) updateClient() *update.Client {
	return &update.Client{Owner: updateRepoOwner, Repo: updateRepoName}
}
