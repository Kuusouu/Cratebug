package modtype

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Community-maintained Marvel Rivals character/skin ID reference, the same
// source BentoMod uses (bentomod/src/character_data.rs).
const characterTableSourceURL = "https://raw.githubusercontent.com/donutman07/MarvelRivalsCharacterIDs/main/MarvelRivalsCharacterIDs.md"

// Default cache freshness window before LoadCharacterTable refetches.
const DefaultCharacterTableMaxAge = 7 * 24 * time.Hour

// Bounds fetchCharacterTable's network round trip so LoadCharacterTable's
// never-fails guarantee can't be defeated by a hung request when the
// caller's ctx carries no deadline of its own.
const characterTableFetchTimeout = 15 * time.Second

// A character/skin ID lookup table for hero-name resolution, sourced from
// characterTableSourceURL (see LoadCharacterTable). The zero value is a
// valid, empty table: ResolveCharacter against it simply resolves nothing.
type CharacterTable struct {
	FetchedAt      time.Time                `json:"fetchedAt"`
	CharacterNames map[string]string        `json:"characterNames"`
	Skins          map[string]SkinReference `json:"skins"`
}

// One skin entry: the character it belongs to and its display name.
type SkinReference struct {
	CharacterID string `json:"characterID"`
	SkinName    string `json:"skinName"`
}

// Returns the default per-user cache location for the character ID table.
func DefaultCharacterTableCachePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "Cratebug", "character-ids.json"), nil
}

// Overridable so tests can supply a table without a network fetch;
// production code always uses fetchCharacterTable.
var fetchCharacterTableFunc = fetchCharacterTable

// Returns a character/skin ID table for hero-name resolution, refreshing it
// from characterTableSourceURL when the cache at cachePath is missing or
// older than maxAge.
//
// LoadCharacterTable never fails: a fetch failure falls back to a stale
// cache, and no usable cache falls back to an empty table. Character data
// being unavailable must degrade to ResolveCharacter finding nothing, not
// an error the caller has to handle.
func LoadCharacterTable(ctx context.Context, cachePath string, maxAge time.Duration) CharacterTable {
	cached, cacheErr := readCharacterTableCache(cachePath)
	if cacheErr == nil && time.Since(cached.FetchedAt) < maxAge {
		return cached
	}

	fetched, err := fetchCharacterTableFunc(ctx)
	if err != nil {
		if cacheErr == nil {
			return cached
		}
		return CharacterTable{}
	}

	// Best-effort: a cache-write failure must not block using the table
	// that was just fetched successfully.
	_ = writeCharacterTableCache(cachePath, fetched)
	return fetched
}

func readCharacterTableCache(cachePath string) (CharacterTable, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return CharacterTable{}, err
	}

	var table CharacterTable
	if err := json.Unmarshal(data, &table); err != nil {
		return CharacterTable{}, fmt.Errorf("decode cached character table: %w", err)
	}
	return table, nil
}

func writeCharacterTableCache(cachePath string, table CharacterTable) error {
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encode character table cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return fmt.Errorf("create character table cache directory: %w", err)
	}
	return os.WriteFile(cachePath, data, 0o600)
}

func fetchCharacterTable(ctx context.Context) (CharacterTable, error) {
	ctx, cancel := context.WithTimeout(ctx, characterTableFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, characterTableSourceURL, nil)
	if err != nil {
		return CharacterTable{}, fmt.Errorf("build character table request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CharacterTable{}, fmt.Errorf("fetch character table: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CharacterTable{}, fmt.Errorf("fetch character table: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CharacterTable{}, fmt.Errorf("read character table response: %w", err)
	}

	table := parseCharacterMarkdown(string(body))
	table.FetchedAt = time.Now()
	return table, nil
}

var (
	characterRowIDPattern = regexp.MustCompile(`^\d{4}$`)
	skinIDPattern         = regexp.MustCompile(`^\d{7}$`)
)

// Parses the community-maintained character ID markdown table. Each row is
// "| ID | NAME | SKIN ID | SKIN NAME |"; a row with a numeric ID and a name
// starts a new character, and following rows with blank ID/NAME cells add
// more skins for that same character until the next non-blank ID cell.
// Malformed rows (missing cells, non-numeric IDs such as unreleased
// characters marked "????") are skipped rather than treated as errors,
// since this is a best-effort community data source, not a contract.
func parseCharacterMarkdown(markdown string) CharacterTable {
	table := CharacterTable{
		CharacterNames: make(map[string]string),
		Skins:          make(map[string]SkinReference),
	}

	var currentCharacterID string
	for _, line := range strings.Split(markdown, "\n") {
		cells := parseTableRow(line)
		if cells == nil {
			continue
		}

		if id := cells[0]; id != "" {
			if characterRowIDPattern.MatchString(id) {
				currentCharacterID = id
				table.CharacterNames[currentCharacterID] = cells[1]
			} else {
				currentCharacterID = ""
			}
		}

		if currentCharacterID == "" || len(cells) < 4 {
			continue
		}

		skinID, skinName := cells[2], cells[3]
		if skinIDPattern.MatchString(skinID) && skinName != "" {
			table.Skins[skinID] = SkinReference{CharacterID: currentCharacterID, SkinName: skinName}
		}
	}
	return table
}

// Splits one markdown table row into trimmed cells, or returns nil if line
// is not a data row (not a table row at all, a separator row, or the
// header row).
func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return nil
	}

	cells := strings.Split(line, "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(cell)
	}
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	if len(cells) < 2 || strings.EqualFold(cells[0], "ID") || isSeparatorRow(cells) {
		return nil
	}
	return cells
}

// Reports whether every cell is empty or made up only of ":" and "-", the
// markdown table alignment-separator row (e.g. "| :--: | :--: |").
func isSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, ":-") != "" {
			return false
		}
	}
	return true
}
