package modtype

import "testing"

func hulkTable() CharacterTable {
	return CharacterTable{
		CharacterNames: map[string]string{"1011": "Hulk"},
		Skins:          map[string]SkinReference{"1011100": {CharacterID: "1011", SkinName: "Mighty G-Bomb"}},
	}
}

func TestResolveCharacterMatchesFolderCharacterID(t *testing.T) {
	// Act
	id, name, skinID, skin := ResolveCharacter(hulkTable(), []string{"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"})

	// Assert
	if id != "1011" || name != "Hulk" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (1011, Hulk, \"\", \"\")", id, name, skinID, skin)
	}
}

func TestResolveCharacterMatchesSkinID(t *testing.T) {
	// Act
	id, name, skinID, skin := ResolveCharacter(hulkTable(), []string{"Marvel/Content/Marvel/Characters/1011/1011100/Meshes/SK_Hulk.uasset"})

	// Assert
	if id != "1011" || name != "Hulk" || skinID != "1011100" || skin != "Mighty G-Bomb" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (1011, Hulk, 1011100, Mighty G-Bomb)", id, name, skinID, skin)
	}
}

func TestResolveCharacterFallsBackToFilenamePattern(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1044": "SomeHero"}}

	// Act
	id, name, _, _ := ResolveCharacter(table, []string{"Audio/vo_1044001_Line.wav"})

	// Assert
	if id != "1044" || name != "SomeHero" {
		t.Errorf("ResolveCharacter() id = %q, name = %q, want 1044, SomeHero", id, name)
	}
}

func TestResolveCharacterFilenameFallbackDoesNotMatchLongerDigitRuns(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1011": "SomeHero"}}

	// Act
	id, name, _, _ := ResolveCharacter(table, []string{"Audio/vo_101112345_Line.wav"})

	// Assert
	if id != "" || name != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\") (an 8-digit run should not match as a 4+3 digit character/skin ID)", id, name)
	}
}

func TestResolveCharacterIgnoresFilenameFallbackForMaterialFiles(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1044": "SomeHero"}}

	// Act
	id, name, _, _ := ResolveCharacter(table, []string{"VFX/MI_1044001_Effect.uasset"})

	// Assert
	if id != "" || name != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\") (mi_ files should not trigger the filename fallback)", id, name)
	}
}

func TestResolveCharacterReturnsEmptyForMultipleCharacters(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1011": "Hulk", "1014": "Punisher"}}

	// Act
	id, name, skinID, skin := ResolveCharacter(table, []string{
		"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset",
		"Marvel/Content/Marvel/Characters/1014/Meshes/SK_Punisher.uasset",
	})

	// Assert
	if id != "" || name != "" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (\"\", \"\", \"\", \"\") for an ambiguous multi-character mod", id, name, skinID, skin)
	}
}

func TestResolveCharacterMultiSkinPreservesCharacterAndClearsSkin(t *testing.T) {
	// Arrange
	table := CharacterTable{
		CharacterNames: map[string]string{"1011": "Hulk"},
		Skins: map[string]SkinReference{
			"1011100": {CharacterID: "1011", SkinName: "Mighty G-Bomb"},
			"1011300": {CharacterID: "1011", SkinName: "Maestro"},
		},
	}

	// Act
	id, name, skinID, skin := ResolveCharacter(table, []string{
		"Marvel/Content/Marvel/Characters/1011/1011100/Meshes/SK_Hulk.uasset",
		"Marvel/Content/Marvel/Characters/1011/1011300/Meshes/SK_Maestro.uasset",
	})

	// Assert: character is resolved unambiguously, skin is cleared as ambiguous
	if id != "1011" || name != "Hulk" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (1011, Hulk, \"\", \"\") for multi-skin pack", id, name, skinID, skin)
	}
}

func TestResolveCharacterReturnsEmptyWhenCharacterIDUnknownToTable(t *testing.T) {
	// Act
	id, name, skinID, skin := ResolveCharacter(CharacterTable{}, []string{"Marvel/Content/Marvel/Characters/1099/Meshes/SK_Unknown.uasset"})

	// Assert
	if id != "" || name != "" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (\"\", \"\", \"\", \"\") when the character ID is not in the table", id, name, skinID, skin)
	}
}

func TestResolveCharacterReturnsEmptyForNoMatch(t *testing.T) {
	// Act
	id, name, skinID, skin := ResolveCharacter(CharacterTable{}, []string{"Random/Unrelated/Asset.uasset"})

	// Assert
	if id != "" || name != "" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (\"\", \"\", \"\", \"\")", id, name, skinID, skin)
	}
}

func TestResolveCharacterReturnsEmptyForNoPaths(t *testing.T) {
	// Act
	id, name, skinID, skin := ResolveCharacter(hulkTable(), nil)

	// Assert
	if id != "" || name != "" || skinID != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q, %q, %q), want (\"\", \"\", \"\", \"\")", id, name, skinID, skin)
	}
}
