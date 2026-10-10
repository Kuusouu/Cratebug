package install

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

// Supported archive extensions for mod intake.
var archiveExtensions = []string{
	".zip",
	".7z",
	".rar",
	".tar",
	".tar.gz",
	".tgz",
	".tar.bz2",
	".tbz2",
	".tar.xz",
	".txz",
}

// Unreal mod bundle extensions that must NOT be treated as archives to unpack.
var bundleExtensions = []string{
	".pak",
	".pak_crateoff",
	".bak_bento",
	".pak_disabled",
	".utoc",
	".ucas",
}

const (
	// Standard permissions for creating extracted directories.
	defaultArchiveDirPermissions fs.FileMode = 0755

	// Standard permissions for extracted regular files.
	defaultArchiveFilePermissions fs.FileMode = 0644
)

// Reports whether a file name carries one of the recognized Unreal bundle extensions
// (primary .pak, its disabled-suffix variants, or a .utoc/.ucas sidecar).
func hasBundleExtension(fileName string) bool {
	lower := strings.ToLower(fileName)
	for _, bundleExt := range bundleExtensions {
		if strings.HasSuffix(lower, bundleExt) {
			return true
		}
	}
	return false
}

// Reports whether a file path has an archive extension and is not a raw Unreal bundle.
func IsArchiveFile(filePath string) bool {
	if hasBundleExtension(filePath) {
		return false
	}
	lower := strings.ToLower(filePath)
	for _, archExt := range archiveExtensions {
		if strings.HasSuffix(lower, archExt) {
			return true
		}
	}
	return false
}

// Extracts the contents of archivePath into destDir safely.
// It defends against Zip Slip / path traversal, rejects symlinks and hardlinks,
// and ensures regular files are written only within destDir.
func ExtractArchive(ctx context.Context, archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer file.Close()

	format, reader, err := archives.Identify(ctx, archivePath, file)
	if err != nil {
		return fmt.Errorf("identify archive format for %q: %w", archivePath, err)
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("archive format %T does not support extraction for %q", format, archivePath)
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve destination directory: %w", err)
	}

	handler := func(ctx context.Context, info archives.FileInfo) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		nameInArchive := info.NameInArchive
		if nameInArchive == "" {
			nameInArchive = info.Name()
		}

		// Reject symlinks and hard links for safety.
		if info.Mode()&fs.ModeSymlink != 0 || info.LinkTarget != "" {
			return fmt.Errorf("archive entry %q contains an unsafe link", nameInArchive)
		}

		// Normalize to forward slashes then clean to evaluate relative path.
		cleanedRel := filepath.Clean(filepath.FromSlash(nameInArchive))
		if !filepath.IsLocal(cleanedRel) {
			return fmt.Errorf("archive entry %q escapes destination directory", nameInArchive)
		}

		targetPath := filepath.Join(destAbs, cleanedRel)
		relToDest, err := filepath.Rel(destAbs, targetPath)
		if err != nil || !filepath.IsLocal(relToDest) || strings.HasPrefix(relToDest, "..") {
			return fmt.Errorf("archive entry %q escapes destination directory", nameInArchive)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, defaultArchiveDirPermissions)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), defaultArchiveDirPermissions); err != nil {
			return fmt.Errorf("create parent directory for %q: %w", cleanedRel, err)
		}

		srcFile, err := info.Open()
		if err != nil {
			return fmt.Errorf("open archive entry %q: %w", nameInArchive, err)
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultArchiveFilePermissions)
		if err != nil {
			return fmt.Errorf("create extracted file %q: %w", cleanedRel, err)
		}

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			dstFile.Close()
			return fmt.Errorf("write extracted file %q: %w", cleanedRel, err)
		}

		if err := dstFile.Close(); err != nil {
			return fmt.Errorf("flush extracted file %q: %w", cleanedRel, err)
		}

		return nil
	}

	return extractor.Extract(ctx, reader, handler)
}
