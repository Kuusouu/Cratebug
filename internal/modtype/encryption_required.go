package modtype

import (
	"strings"

	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

// Longer prefixes first so Marvel/Content/Marvel/ wins over Marvel/Content/.
// These include the classify content roots plus the shorter stems that show
// up in some listings (Marvel/Content/Characters, /Game/WwiseAudio).
var encryptionRootPrefixes = []string{
	"Marvel/Content/Marvel/",
	"/Game/Marvel/",
	"Marvel/Content/",
	"/Game/",
}

const charactersFolder = "characters"

// True when a complete IoStore listing includes any real asset outside
// /Game/Marvel/Characters. Those containers will not load unless they are
// obfuscated with the game AES key. Empty listings and companion metadata
// names do not force encryption.
func RequiresIoStoreEncryption(paths []string) bool {
	for _, path := range paths {
		if path == "" || uassettool.IsCompanionMetadataPath(path) {
			continue
		}
		if !pathUnderCharacters(path) {
			return true
		}
	}
	return false
}

func pathUnderCharacters(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	stripped := strings.TrimPrefix(stripEncryptionRoot(normalized), "/")
	lower := strings.ToLower(stripped)
	return lower == charactersFolder || strings.HasPrefix(lower, charactersFolder+"/")
}

func stripEncryptionRoot(path string) string {
	for _, prefix := range encryptionRootPrefixes {
		if after, ok := cutPrefixFold(path, prefix); ok {
			return after
		}
	}
	return path
}

func cutPrefixFold(value, prefix string) (string, bool) {
	if len(value) < len(prefix) {
		return "", false
	}
	if strings.EqualFold(value[:len(prefix)], prefix) {
		return value[len(prefix):], true
	}
	return "", false
}
