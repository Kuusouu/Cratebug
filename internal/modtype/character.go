package modtype

import (
	"regexp"
	"strings"
)

var (
	// A folder segment naming a hero, e.g. ".../Characters/1011/...".
	characterFolderPattern = regexp.MustCompile(`(?:Characters|Hero|Hero_ST)/(\d{4})`)
	// A character folder directly containing a skin-ID folder, e.g. "1011/1011100".
	skinFolderPattern = regexp.MustCompile(`(\d{4})/(\d{7})`)
	// Filename fallback: a 7-digit ID split as a 4-digit character ID (in
	// Marvel Rivals's known 101x-106x range, to reduce false positives from
	// arbitrary numbers) plus a 3-digit skin suffix, e.g. "vo_1044001".
	characterFilenameFallbackPattern = regexp.MustCompile(`[_/](10[1-6]\d)(\d{3})(?:\D|$)`)
)

// Resolves a mod's hero ID, hero name, skin ID, and skin display name from its internal asset
// paths, adapted from BentoMod's own regex-based matching
// (bentomod/src/utils.rs:12-28,83-110,223-263) to Cratebug's forward-slash
// path convention. Returns empty strings, not an error, when nothing
// resolves, when the resolved character ID is not in table, or when more
// than one character is found across the mod's paths — an ambiguous mod
// has no single hero name to report.
func ResolveCharacter(table CharacterTable, paths []string) (characterID, characterName, skinID, skinName string) {
	characterIDs := make(map[string]struct{})
	skinIDs := make(map[string]struct{})

	for _, path := range paths {
		matchedFolder := false
		if match := characterFolderPattern.FindStringSubmatch(path); match != nil {
			characterIDs[match[1]] = struct{}{}
			matchedFolder = true
		}
		if match := skinFolderPattern.FindStringSubmatch(path); match != nil {
			skinIDs[match[2]] = struct{}{}
		}

		// The filename fallback is skipped for material files (mi_ prefix),
		// which commonly contain unrelated numeric sequences, and for any
		// path that already matched the safer folder-based pattern.
		if !matchedFolder && !isMaterialFile(path) {
			if match := characterFilenameFallbackPattern.FindStringSubmatch(path); match != nil {
				characterIDs[match[1]] = struct{}{}
			}
		}
	}

	if len(characterIDs) != 1 {
		return "", "", "", ""
	}
	var resolvedID string
	for id := range characterIDs {
		resolvedID = id
	}

	characterName = table.CharacterNames[resolvedID]
	if characterName == "" {
		return "", "", "", ""
	}

	var matchingSkins []string
	for skinIDKey := range skinIDs {
		if skin, ok := table.Skins[skinIDKey]; ok && skin.CharacterID == resolvedID {
			matchingSkins = append(matchingSkins, skinIDKey)
		}
	}
	if len(matchingSkins) == 1 {
		singleSkinID := matchingSkins[0]
		return resolvedID, characterName, singleSkinID, table.Skins[singleSkinID].SkinName
	}
	return resolvedID, characterName, "", ""
}

func isMaterialFile(path string) bool {
	filename := path
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		filename = path[slash+1:]
	}
	return strings.HasPrefix(strings.ToLower(filename), "mi_")
}
