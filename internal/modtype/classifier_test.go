package modtype

import "testing"

func TestClassifyResolvesEachCategory(t *testing.T) {
	cases := []struct {
		name string
		path string
		want Category
	}{
		{"audio", "WwiseAudio/Media/Sound.bnk", CategoryAudio},
		{"movies", "Movies/Intro.bik", CategoryMovies},
		{"ui", "UI/Icons/Icon.uasset", CategoryUI},
		{"skeletal mesh", "Characters/1032/Meshes/SK_Hero.uasset", CategoryMesh},
		{"static mesh", "Props/SM_Crate.uasset", CategoryStaticMesh},
		{"vfx material", "VFX/MI_Fire.uasset", CategoryVFX},
		{"texture", "Textures/T_Skin.uasset", CategoryTexture},
		{"blueprint prefix", "Blueprints/BP_Ability.uasset", CategoryBlueprint},
		{"blueprint generated class", "Characters/Hero_C.uasset", CategoryBlueprint},
		{"text stringtable", "Localization/StringTable/Strings.uasset", CategoryText},
		{"unrecognized path", "Random/Unclassified.uasset", CategoryUnknown},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := Classify([]string{testCase.path})

			// Assert
			if got != testCase.want {
				t.Errorf("Classify([%q]) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

func TestClassifyReturnsUnknownForEmptyPathList(t *testing.T) {
	// Act
	got := Classify(nil)

	// Assert
	if got != CategoryUnknown {
		t.Errorf("Classify(nil) = %q, want %q", got, CategoryUnknown)
	}
}

// Skeletal mesh takes priority over texture whenever both are present,
// matching BentoMod's category resolution order (utils.rs:272-296).
func TestClassifyPrioritizesMeshOverTexture(t *testing.T) {
	// Act
	got := Classify([]string{"Characters/SK_Hero.uasset", "Textures/T_Skin.uasset"})

	// Assert
	if got != CategoryMesh {
		t.Errorf("Classify() = %q, want %q", got, CategoryMesh)
	}
}

// Audio alone (no mesh, static mesh, texture, or material content) is a
// "pure" audio mod.
func TestClassifyReturnsAudioForPureAudio(t *testing.T) {
	// Act
	got := Classify([]string{"WwiseAudio/Media/Sound.bnk"})

	// Assert
	if got != CategoryAudio {
		t.Errorf("Classify() = %q, want %q", got, CategoryAudio)
	}
}

// Skeletal mesh has unconditional priority over audio: a mod with both
// audio and a skeletal mesh is a Mesh mod, not a "mixed" Audio mod. This is
// the case most likely to get inverted during a hasty port, since it looks
// superficially similar to the genuinely mixed case below.
func TestClassifyPrioritizesMeshOverMixedAudio(t *testing.T) {
	// Act
	got := Classify([]string{"WwiseAudio/Media/Sound.bnk", "Characters/SK_Hero.uasset"})

	// Assert
	if got != CategoryMesh {
		t.Errorf("Classify() = %q, want %q", got, CategoryMesh)
	}
}

// Audio combined with texture (but no mesh, static mesh, or material) is
// the actual "mixed" audio case (utils.rs:272-296, rule 7 vs rule 8): audio
// still wins over texture, since it is checked first once the "pure"
// categories are ruled out.
func TestClassifyPrioritizesMixedAudioOverTexture(t *testing.T) {
	// Act
	got := Classify([]string{"WwiseAudio/Media/Sound.bnk", "Textures/T_Skin.uasset"})

	// Assert
	if got != CategoryAudio {
		t.Errorf("Classify() = %q, want %q", got, CategoryAudio)
	}
}

func TestClassifyStripsKnownContentRootPrefixes(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"marvel content prefix", "Marvel/Content/Marvel/Characters/SK_Hero.uasset"},
		{"game marvel prefix", "/Game/Marvel/Characters/SK_Hero.uasset"},
		{"unprefixed", "Characters/SK_Hero.uasset"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Act
			got := Classify([]string{testCase.path})

			// Assert
			if got != CategoryMesh {
				t.Errorf("Classify([%q]) = %q, want %q", testCase.path, got, CategoryMesh)
			}
		})
	}
}

// IoStore listings commonly report package paths with no file extension;
// these must still trip the mesh/texture/blueprint prefix checks.
func TestClassifyTreatsExtensionlessPathAsUasset(t *testing.T) {
	// Act
	got := Classify([]string{"Characters/1032/Meshes/SK_Hero"})

	// Assert
	if got != CategoryMesh {
		t.Errorf("Classify() = %q, want %q", got, CategoryMesh)
	}
}

func TestClassifyIsCaseInsensitive(t *testing.T) {
	// Act
	got := Classify([]string{"CHARACTERS/SK_HERO.UASSET"})

	// Assert
	if got != CategoryMesh {
		t.Errorf("Classify() = %q, want %q", got, CategoryMesh)
	}
}
