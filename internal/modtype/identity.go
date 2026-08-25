package modtype

import "github.com/Kuusouu/Cratebug/internal/discovery"

// Extends Determine's category with a resolved hero ID, hero name, skin ID, and skin name,
// where the same internal path listing makes one determinable. CharacterID, CharacterName,
// SkinID, and SkinName are empty when unresolved: absence of a hero name is not an error.
type Identity struct {
	Category      Category `json:"category"`
	CharacterID   string   `json:"characterID"`
	CharacterName string   `json:"characterName"`
	SkinID        string   `json:"skinID"`
	SkinName      string   `json:"skinName"`
}

// Resolves entry's category and, where possible, its hero ID, hero name, skin ID, and skin name,
// from one internal-path listing call — no UAssetToolRivals call beyond
// what Determine itself performs. table is typically loaded once per
// session via LoadCharacterTable and reused across calls, since loading it
// may involve a network fetch.
//
// DetermineIdentity returns ErrCannotDetermineType under the same
// conditions as Determine. A missing or stale character table does not
// cause an error: CharacterID, CharacterName, SkinID, and SkinName are simply empty, so a
// character-data problem never costs the caller an otherwise successful
// category result.
func DetermineIdentity(c caller, root string, entry discovery.Entry, table CharacterTable) (Identity, error) {
	paths, err := listInternalPaths(c, root, entry)
	if err != nil {
		return Identity{}, err
	}

	characterID, characterName, skinID, skinName := ResolveCharacter(table, paths)
	return Identity{
		Category:      Classify(paths),
		CharacterID:   characterID,
		CharacterName: characterName,
		SkinID:        skinID,
		SkinName:      skinName,
	}, nil
}
