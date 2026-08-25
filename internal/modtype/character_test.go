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
	name, skin := ResolveCharacter(hulkTable(), []string{"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"})

	// Assert
	if name != "Hulk" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (Hulk, \"\")", name, skin)
	}
}

func TestResolveCharacterMatchesSkinID(t *testing.T) {
	// Act
	name, skin := ResolveCharacter(hulkTable(), []string{"Marvel/Content/Marvel/Characters/1011/1011100/Meshes/SK_Hulk.uasset"})

	// Assert
	if name != "Hulk" || skin != "Mighty G-Bomb" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (Hulk, Mighty G-Bomb)", name, skin)
	}
}

func TestResolveCharacterFallsBackToFilenamePattern(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1044": "SomeHero"}}

	// Act
	name, _ := ResolveCharacter(table, []string{"Audio/vo_1044001_Line.wav"})

	// Assert
	if name != "SomeHero" {
		t.Errorf("ResolveCharacter() name = %q, want SomeHero", name)
	}
}

func TestResolveCharacterFilenameFallbackDoesNotMatchLongerDigitRuns(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1011": "SomeHero"}}

	// Act
	name, _ := ResolveCharacter(table, []string{"Audio/vo_101112345_Line.wav"})

	// Assert
	if name != "" {
		t.Errorf("ResolveCharacter() name = %q, want \"\" (an 8-digit run should not match as a 4+3 digit character/skin ID)", name)
	}
}

func TestResolveCharacterIgnoresFilenameFallbackForMaterialFiles(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1044": "SomeHero"}}

	// Act
	name, _ := ResolveCharacter(table, []string{"VFX/MI_1044001_Effect.uasset"})

	// Assert
	if name != "" {
		t.Errorf("ResolveCharacter() name = %q, want \"\" (mi_ files should not trigger the filename fallback)", name)
	}
}

func TestResolveCharacterReturnsEmptyForMultipleCharacters(t *testing.T) {
	// Arrange
	table := CharacterTable{CharacterNames: map[string]string{"1011": "Hulk", "1014": "Punisher"}}

	// Act
	name, skin := ResolveCharacter(table, []string{
		"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset",
		"Marvel/Content/Marvel/Characters/1014/Meshes/SK_Punisher.uasset",
	})

	// Assert
	if name != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\") for an ambiguous multi-character mod", name, skin)
	}
}

func TestResolveCharacterReturnsEmptyWhenCharacterIDUnknownToTable(t *testing.T) {
	// Act
	name, skin := ResolveCharacter(CharacterTable{}, []string{"Marvel/Content/Marvel/Characters/1099/Meshes/SK_Unknown.uasset"})

	// Assert
	if name != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\") when the character ID is not in the table", name, skin)
	}
}

func TestResolveCharacterReturnsEmptyForNoMatch(t *testing.T) {
	// Act
	name, skin := ResolveCharacter(CharacterTable{}, []string{"Random/Unrelated/Asset.uasset"})

	// Assert
	if name != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\")", name, skin)
	}
}

func TestResolveCharacterReturnsEmptyForNoPaths(t *testing.T) {
	// Act
	name, skin := ResolveCharacter(hulkTable(), nil)

	// Assert
	if name != "" || skin != "" {
		t.Errorf("ResolveCharacter() = (%q, %q), want (\"\", \"\")", name, skin)
	}
}
