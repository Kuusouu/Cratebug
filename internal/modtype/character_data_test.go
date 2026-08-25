package modtype

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

const sampleCharacterMarkdown = `# Marvel Rivals Character IDs

|  ID  | NAME | SKIN IDs | SKIN NAMES
| :--: | :--: | :--: | :--: |
| 1011 | Hulk | 1011100 | Mighty G-Bomb |
| | | 1011300 | Maestro |
| 1014 | Punisher | 1014100 | Camo |
| ???? | Upcoming Characters | | |
| 9999 | Hero Zero | | |
| ???? | Gorr The God Butcher |
`

func TestParseCharacterMarkdownExtractsCharactersAndSkins(t *testing.T) {
	// Act
	table := parseCharacterMarkdown(sampleCharacterMarkdown)

	// Assert
	if table.CharacterNames["1011"] != "Hulk" {
		t.Errorf("CharacterNames[1011] = %q, want Hulk", table.CharacterNames["1011"])
	}
	if table.CharacterNames["1014"] != "Punisher" {
		t.Errorf("CharacterNames[1014] = %q, want Punisher", table.CharacterNames["1014"])
	}
	if skin, ok := table.Skins["1011100"]; !ok || skin.CharacterID != "1011" || skin.SkinName != "Mighty G-Bomb" {
		t.Errorf("Skins[1011100] = %+v, ok=%v, want {1011, Mighty G-Bomb}", skin, ok)
	}
	if skin, ok := table.Skins["1011300"]; !ok || skin.CharacterID != "1011" || skin.SkinName != "Maestro" {
		t.Errorf("Skins[1011300] = %+v, ok=%v, want {1011, Maestro} (a continuation row)", skin, ok)
	}
}

// Rows with a non-numeric ID (unreleased characters marked "????"), non-playable
// test IDs (such as 9999), and stale "(Old)" collision rows must not produce bogus entries.
func TestParseCharacterMarkdownSkipsUnreleasedAndMalformedRows(t *testing.T) {
	markdown := `# Marvel Rivals Character IDs

|  ID  | NAME | SKIN IDs | SKIN NAMES
| :--: | :--: | :--: | :--: |
| 1062 | Devil Dinosaur | 1062100 | TROPICAL BEAST |
| 1063 | Cyclops | 1063100 | SHADOWED NEMESIS |
| ???? | Upcoming Characters | | |
| 9999 | Hero Zero | | |
| 1062 | Beast (Old) | | |
| 1063 | Nightcrawler (Old) | | |
`
	// Act
	table := parseCharacterMarkdown(markdown)

	// Assert
	if _, ok := table.CharacterNames["????"]; ok {
		t.Errorf("CharacterNames contains the literal placeholder %q, want it skipped", "????")
	}
	if _, ok := table.CharacterNames["9999"]; ok {
		t.Errorf("CharacterNames contains non-playable test ID 9999, want it skipped")
	}
	if table.CharacterNames["1062"] != "Devil Dinosaur" {
		t.Errorf("CharacterNames[1062] = %q, want Devil Dinosaur (not overwritten by Beast (Old))", table.CharacterNames["1062"])
	}
	if table.CharacterNames["1063"] != "Cyclops" {
		t.Errorf("CharacterNames[1063] = %q, want Cyclops (not overwritten by Nightcrawler (Old))", table.CharacterNames["1063"])
	}
}

func TestParseCharacterMarkdownReturnsEmptyTableForEmptyInput(t *testing.T) {
	// Act
	table := parseCharacterMarkdown("")

	// Assert
	if len(table.CharacterNames) != 0 || len(table.Skins) != 0 {
		t.Errorf("parseCharacterMarkdown(\"\") = %+v, want an empty table", table)
	}
}

func TestCharacterTableCacheRoundTrips(t *testing.T) {
	// Arrange
	cachePath := filepath.Join(t.TempDir(), "character-ids.json")
	want := CharacterTable{
		FetchedAt:      time.Now().Truncate(time.Second),
		CharacterNames: map[string]string{"1011": "Hulk"},
		Skins:          map[string]SkinReference{"1011100": {CharacterID: "1011", SkinName: "Mighty G-Bomb"}},
	}

	// Act
	if err := writeCharacterTableCache(cachePath, want); err != nil {
		t.Fatalf("writeCharacterTableCache() error = %v", err)
	}
	got, err := readCharacterTableCache(cachePath)

	// Assert
	if err != nil {
		t.Fatalf("readCharacterTableCache() error = %v", err)
	}
	if got.CharacterNames["1011"] != "Hulk" || got.Skins["1011100"].SkinName != "Mighty G-Bomb" {
		t.Errorf("readCharacterTableCache() = %+v, want it to match what was written", got)
	}
}

func withFakeCharacterFetch(t *testing.T, fetch func(context.Context) (CharacterTable, error)) {
	t.Helper()
	original := fetchCharacterTableFunc
	fetchCharacterTableFunc = fetch
	t.Cleanup(func() { fetchCharacterTableFunc = original })
}

func TestLoadCharacterTableReusesAFreshCacheWithoutFetching(t *testing.T) {
	// Arrange
	cachePath := filepath.Join(t.TempDir(), "character-ids.json")
	cached := CharacterTable{FetchedAt: time.Now(), CharacterNames: map[string]string{"1011": "Hulk"}, Skins: map[string]SkinReference{}}
	if err := writeCharacterTableCache(cachePath, cached); err != nil {
		t.Fatalf("writeCharacterTableCache() error = %v", err)
	}
	fetchCalled := false
	withFakeCharacterFetch(t, func(context.Context) (CharacterTable, error) {
		fetchCalled = true
		return CharacterTable{}, nil
	})

	// Act
	got := LoadCharacterTable(context.Background(), cachePath, time.Hour)

	// Assert
	if fetchCalled {
		t.Errorf("LoadCharacterTable() fetched despite a fresh cache being present")
	}
	if got.CharacterNames["1011"] != "Hulk" {
		t.Errorf("LoadCharacterTable() = %+v, want the cached table", got)
	}
}

func TestLoadCharacterTableFetchesAndCachesWhenStale(t *testing.T) {
	// Arrange
	cachePath := filepath.Join(t.TempDir(), "character-ids.json")
	stale := CharacterTable{FetchedAt: time.Now().Add(-30 * 24 * time.Hour), CharacterNames: map[string]string{"1011": "Hulk"}, Skins: map[string]SkinReference{}}
	if err := writeCharacterTableCache(cachePath, stale); err != nil {
		t.Fatalf("writeCharacterTableCache() error = %v", err)
	}
	fresh := CharacterTable{CharacterNames: map[string]string{"1014": "Punisher"}, Skins: map[string]SkinReference{}}
	withFakeCharacterFetch(t, func(context.Context) (CharacterTable, error) {
		return fresh, nil
	})

	// Act
	got := LoadCharacterTable(context.Background(), cachePath, DefaultCharacterTableMaxAge)

	// Assert
	if got.CharacterNames["1014"] != "Punisher" {
		t.Errorf("LoadCharacterTable() = %+v, want the freshly fetched table", got)
	}
	onDisk, err := readCharacterTableCache(cachePath)
	if err != nil || onDisk.CharacterNames["1014"] != "Punisher" {
		t.Errorf("cache at %s = %+v, err=%v, want it updated with the fetched table", cachePath, onDisk, err)
	}
}

func TestLoadCharacterTableFallsBackToStaleCacheOnFetchFailure(t *testing.T) {
	// Arrange
	cachePath := filepath.Join(t.TempDir(), "character-ids.json")
	stale := CharacterTable{FetchedAt: time.Now().Add(-30 * 24 * time.Hour), CharacterNames: map[string]string{"1011": "Hulk"}, Skins: map[string]SkinReference{}}
	if err := writeCharacterTableCache(cachePath, stale); err != nil {
		t.Fatalf("writeCharacterTableCache() error = %v", err)
	}
	withFakeCharacterFetch(t, func(context.Context) (CharacterTable, error) {
		return CharacterTable{}, errors.New("network unreachable")
	})

	// Act
	got := LoadCharacterTable(context.Background(), cachePath, DefaultCharacterTableMaxAge)

	// Assert
	if got.CharacterNames["1011"] != "Hulk" {
		t.Errorf("LoadCharacterTable() = %+v, want the stale cache as a fallback", got)
	}
}

func TestLoadCharacterTableReturnsEmptyTableWhenNoCacheAndFetchFails(t *testing.T) {
	// Arrange
	cachePath := filepath.Join(t.TempDir(), "character-ids.json")
	withFakeCharacterFetch(t, func(context.Context) (CharacterTable, error) {
		return CharacterTable{}, errors.New("network unreachable")
	})

	// Act
	got := LoadCharacterTable(context.Background(), cachePath, DefaultCharacterTableMaxAge)

	// Assert
	if len(got.CharacterNames) != 0 || len(got.Skins) != 0 {
		t.Errorf("LoadCharacterTable() = %+v, want an empty table, not an error", got)
	}
}
