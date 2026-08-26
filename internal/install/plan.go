package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// CollisionInfo records conflict details when a staged mod collides with an existing library entry.
type CollisionInfo struct {
	HasCollision   bool     `json:"hasCollision"`
	ExistingModID  string   `json:"existingModID,omitempty"`
	CollidingFiles []string `json:"collidingFiles,omitempty"`
	Description    string   `json:"description,omitempty"`
}

// PreviewItem represents one staged mod candidate presented to the user before installation.
type PreviewItem struct {
	ID                string                 `json:"id"`
	ModName           string                 `json:"modName"`
	OriginalStem      string                 `json:"originalStem"`
	BundleFormat      discovery.BundleFormat `json:"bundleFormat"`
	Files             []string               `json:"files"`
	TotalSizeBytes    int64                  `json:"totalSizeBytes"`
	DestinationFolder string                 `json:"destinationFolder"`
	Collision         CollisionInfo          `json:"collision"`
}

// PreviewResult contains all discovered mods in a staging session with collision checks.
type PreviewResult struct {
	SessionID string        `json:"sessionId"`
	Items     []PreviewItem `json:"items"`
}

// BuildPreview constructs the installation preview and detects collisions against modRoot.
func BuildPreview(modRoot string, session *StagedSession, defaultFolder string) (PreviewResult, error) {
	if session == nil {
		return PreviewResult{}, fmt.Errorf("staging session is nil")
	}

	normDefaultFolder := filepath.ToSlash(filepath.Clean(filepath.FromSlash(defaultFolder)))
	if normDefaultFolder == "." || normDefaultFolder == "/" {
		normDefaultFolder = ""
	}
	if normDefaultFolder != "" && (!filepath.IsLocal(normDefaultFolder) || strings.HasPrefix(normDefaultFolder, "..")) {
		return PreviewResult{}, fmt.Errorf("default destination folder %q escapes mod root", defaultFolder)
	}

	var library discovery.Library
	if modRoot != "" {
		var err error
		library, err = discovery.Scan(modRoot)
		if err != nil {
			return PreviewResult{}, fmt.Errorf("scan mod library for collision check: %w", err)
		}
	}

	var previewItems []PreviewItem
	for _, mod := range session.Mods {
		var fileNames []string
		for _, f := range mod.AllFiles {
			fileNames = append(fileNames, filepath.Base(f))
		}

		collision := checkModCollision(modRoot, normDefaultFolder, mod, library)

		previewItems = append(previewItems, PreviewItem{
			ID:                mod.ID,
			ModName:           mod.DisplayName,
			OriginalStem:      mod.Stem,
			BundleFormat:      mod.BundleFormat,
			Files:             fileNames,
			TotalSizeBytes:    mod.TotalSizeBytes,
			DestinationFolder: normDefaultFolder,
			Collision:         collision,
		})
	}

	return PreviewResult{
		SessionID: session.ID,
		Items:     previewItems,
	}, nil
}

// Inspects the destination folder for filename and display-name collisions.
func checkModCollision(modRoot, destFolder string, stagedMod StagedMod, library discovery.Library) CollisionInfo {
	if modRoot == "" {
		return CollisionInfo{}
	}

	var collidingFiles []string
	var existingModID string

	destDirAbs := filepath.Join(modRoot, filepath.FromSlash(destFolder))

	// Check if any expected bundle files already exist on disk in the target folder
	for _, f := range stagedMod.AllFiles {
		baseName := filepath.Base(f)
		targetPath := filepath.Join(destDirAbs, baseName)
		if _, err := os.Lstat(targetPath); err == nil {
			relPath := filepath.ToSlash(filepath.Join(destFolder, baseName))
			collidingFiles = append(collidingFiles, relPath)
		}
	}

	// Check if any library entry in the same folder matches the display name or primary stem
	normDest := strings.ToLower(destFolder)
	for _, entry := range library.Entries {
		if strings.ToLower(entry.RelativeFolder) != normDest {
			continue
		}

		if strings.EqualFold(entry.DisplayName, stagedMod.DisplayName) ||
			strings.EqualFold(extractFileStem(filepath.Base(entry.PrimaryPath)), stagedMod.Stem) {
			existingModID = entry.ID
			if len(collidingFiles) == 0 && entry.PrimaryPath != "" {
				collidingFiles = append(collidingFiles, entry.PrimaryPath)
			}
			break
		}
	}

	if len(collidingFiles) > 0 || existingModID != "" {
		desc := fmt.Sprintf("A mod with matching files or name already exists in '%s'", destFolder)
		if destFolder == "" {
			desc = "A mod with matching files or name already exists in the library root"
		}
		return CollisionInfo{
			HasCollision:   true,
			ExistingModID:  existingModID,
			CollidingFiles: collidingFiles,
			Description:    desc,
		}
	}

	return CollisionInfo{
		HasCollision: false,
	}
}
