package modtype

import (
	"errors"
	"testing"
)

func TestDetermineIdentityCombinesCategoryAndCharacter(t *testing.T) {
	// Arrange
	fake := &fakeCaller{responses: map[string]string{
		"list_pak": `{"files":[{"path":"Marvel/Content/Marvel/Characters/1011/1011100/Meshes/SK_Hulk.uasset"}]}`,
	}}
	entry := classicEntry("Mods/Example.pak")

	// Act
	identity, err := DetermineIdentity(fake, "C:/root", entry, hulkTable())

	// Assert
	if err != nil {
		t.Fatalf("DetermineIdentity() error = %v, want nil", err)
	}
	if identity.Category != CategoryMesh {
		t.Errorf("identity.Category = %q, want %q", identity.Category, CategoryMesh)
	}
	if identity.CharacterID != "1011" || identity.CharacterName != "Hulk" || identity.SkinID != "1011100" || identity.SkinName != "Mighty G-Bomb" {
		t.Errorf("identity = %+v, want CharacterID=1011, CharacterName=Hulk, SkinID=1011100, SkinName=Mighty G-Bomb", identity)
	}
}

// A missing or empty character table must not cost the caller its category
// result: only CharacterName/SkinName go empty.
func TestDetermineIdentityDegradesToNoCharacterNameWithoutLosingCategory(t *testing.T) {
	// Arrange
	fake := &fakeCaller{responses: map[string]string{
		"list_pak": `{"files":[{"path":"Marvel/Content/Marvel/Characters/1011/Meshes/SK_Hulk.uasset"}]}`,
	}}
	entry := classicEntry("Mods/Example.pak")

	// Act
	identity, err := DetermineIdentity(fake, "C:/root", entry, CharacterTable{})

	// Assert
	if err != nil {
		t.Fatalf("DetermineIdentity() error = %v, want nil", err)
	}
	if identity.Category != CategoryMesh {
		t.Errorf("identity.Category = %q, want %q even with no character table", identity.Category, CategoryMesh)
	}
	if identity.CharacterID != "" || identity.CharacterName != "" || identity.SkinID != "" || identity.SkinName != "" {
		t.Errorf("identity = %+v, want empty CharacterID/CharacterName/SkinID/SkinName", identity)
	}
}

func TestDetermineIdentityPropagatesUnderlyingCallError(t *testing.T) {
	// Arrange
	fake := &fakeCaller{errs: map[string]error{"list_pak": errors.New("boom")}}
	entry := classicEntry("Mods/Example.pak")

	// Act
	_, err := DetermineIdentity(fake, "C:/root", entry, CharacterTable{})

	// Assert
	if err == nil {
		t.Fatal("DetermineIdentity() error = nil, want the underlying call error")
	}
}
