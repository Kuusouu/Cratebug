package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetThemeRejectsAnUnsupportedValue(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetTheme("solarized")

	// Assert
	if err == nil {
		t.Fatal("SetTheme() succeeded, want an error for an unsupported theme")
	}
}

func TestSetThemeAcceptsEachSupportedValue(t *testing.T) {
	for _, theme := range []string{"system", "light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			// Arrange
			var doc Document

			// Act
			err := doc.SetTheme(theme)

			// Assert
			if err != nil {
				t.Fatalf("SetTheme(%q) = %v, want no error", theme, err)
			}
			if doc.Settings.Theme != theme {
				t.Errorf("Settings.Theme = %q, want %q", doc.Settings.Theme, theme)
			}
		})
	}
}

func TestSetDefaultViewModeRejectsAnUnsupportedValue(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetDefaultViewMode("carousel")

	// Assert
	if err == nil {
		t.Fatal("SetDefaultViewMode() succeeded, want an error for an unsupported view mode")
	}
}

func TestSetDefaultViewModeAcceptsEachSupportedValue(t *testing.T) {
	for _, mode := range []string{"compact", "large", "list"} {
		t.Run(mode, func(t *testing.T) {
			// Arrange
			var doc Document

			// Act
			err := doc.SetDefaultViewMode(mode)

			// Assert
			if err != nil {
				t.Fatalf("SetDefaultViewMode(%q) = %v, want no error", mode, err)
			}
			if doc.Settings.DefaultViewMode != mode {
				t.Errorf("Settings.DefaultViewMode = %q, want %q", doc.Settings.DefaultViewMode, mode)
			}
		})
	}
}

func TestThemeAndDefaultViewModeSurviveASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	if err := doc.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetDefaultViewMode("list"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.Theme != "dark" {
		t.Errorf("Settings.Theme = %q, want %q", reloaded.Settings.Theme, "dark")
	}
	if reloaded.Settings.DefaultViewMode != "list" {
		t.Errorf("Settings.DefaultViewMode = %q, want %q", reloaded.Settings.DefaultViewMode, "list")
	}
}

func TestSetAccentColorRejectsAnUnsupportedValue(t *testing.T) {
	for _, color := range []string{"orange", "#fff", "#gggggg", "f0a54d", "#f0a54d0"} {
		t.Run(color, func(t *testing.T) {
			// Arrange
			var doc Document

			// Act
			err := doc.SetAccentColor(color)

			// Assert
			if err == nil {
				t.Fatalf("SetAccentColor(%q) succeeded, want an error", color)
			}
		})
	}
}

func TestSetAccentColorAcceptsA6DigitHexValue(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetAccentColor("#f0a54d")

	// Assert
	if err != nil {
		t.Fatalf("SetAccentColor(%q) = %v, want no error", "#f0a54d", err)
	}
	if doc.Settings.AccentColor != "#f0a54d" {
		t.Errorf("Settings.AccentColor = %q, want %q", doc.Settings.AccentColor, "#f0a54d")
	}
}

func TestSetAccentColorAcceptsAnEmptyValueToClearTheOverride(t *testing.T) {
	// Arrange
	doc := Document{Settings: Settings{AccentColor: "#f0a54d"}}

	// Act
	err := doc.SetAccentColor("")

	// Assert
	if err != nil {
		t.Fatalf("SetAccentColor(%q) = %v, want no error", "", err)
	}
	if doc.Settings.AccentColor != "" {
		t.Errorf("Settings.AccentColor = %q, want empty", doc.Settings.AccentColor)
	}
}

func TestAccentColorSurvivesASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	if err := doc.SetAccentColor("#8b5cf6"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.AccentColor != "#8b5cf6" {
		t.Errorf("Settings.AccentColor = %q, want %q", reloaded.Settings.AccentColor, "#8b5cf6")
	}
}

func TestSetLibraryProviderRejectsAnUnknownProvider(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetLibraryProvider("egs")

	// Assert
	if err == nil {
		t.Fatal("SetLibraryProvider() succeeded for an unregistered provider, want an error")
	}
}

func TestSetLibraryProviderAcceptsARegisteredProvider(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetLibraryProvider("steam")

	// Assert
	if err != nil {
		t.Fatalf("SetLibraryProvider() = %v, want no error", err)
	}
	if doc.Settings.LibraryProvider != "steam" {
		t.Errorf("Settings.LibraryProvider = %q, want %q", doc.Settings.LibraryProvider, "steam")
	}
}

func TestSetLibraryProviderAcceptsTheEpicProvider(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	err := doc.SetLibraryProvider("epic")

	// Assert
	if err != nil {
		t.Fatalf("SetLibraryProvider() = %v, want no error", err)
	}
	if doc.Settings.LibraryProvider != "epic" {
		t.Errorf("Settings.LibraryProvider = %q, want %q", doc.Settings.LibraryProvider, "epic")
	}
}

func TestSetLibraryProviderAcceptsAnEmptyValueToRestoreTheDefault(t *testing.T) {
	// Arrange
	doc := Document{Settings: Settings{LibraryProvider: "steam"}}

	// Act
	err := doc.SetLibraryProvider("")

	// Assert
	if err != nil {
		t.Fatalf("SetLibraryProvider(\"\") = %v, want no error", err)
	}
	if doc.Settings.LibraryProvider != "" {
		t.Errorf("Settings.LibraryProvider = %q, want empty", doc.Settings.LibraryProvider)
	}
}

func TestLibraryProviderSurvivesASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	if err := doc.SetLibraryProvider("steam"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.LibraryProvider != "steam" {
		t.Errorf("Settings.LibraryProvider = %q, want %q", reloaded.Settings.LibraryProvider, "steam")
	}
}

func TestDocumentWithNoLibraryProviderLoadsAsEmpty(t *testing.T) {
	// Arrange: a document written before this field existed has no
	// "libraryProvider" key at all, not an empty one.
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	if err := store.Save(Document{}); err != nil {
		t.Fatal(err)
	}

	// Act
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.LibraryProvider != "" {
		t.Errorf("Settings.LibraryProvider = %q, want empty for a document that never set it", reloaded.Settings.LibraryProvider)
	}
}

func TestSetLastSeenVersion(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	doc.SetLastSeenVersion("2026.08.27")

	// Assert
	if doc.Settings.LastSeenVersion != "2026.08.27" {
		t.Errorf("Settings.LastSeenVersion = %q, want %q", doc.Settings.LastSeenVersion, "2026.08.27")
	}
}

func TestLastSeenVersionSurvivesASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	doc.SetLastSeenVersion("2026.08.27")

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.LastSeenVersion != "2026.08.27" {
		t.Errorf("Settings.LastSeenVersion = %q, want %q", reloaded.Settings.LastSeenVersion, "2026.08.27")
	}
}

func TestDocumentWithNoLastSeenVersionLoadsAsEmpty(t *testing.T) {
	// Arrange: a document written before this field existed has no
	// "lastSeenVersion" key at all, not an empty one.
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	if err := store.Save(Document{}); err != nil {
		t.Fatal(err)
	}

	// Act
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.LastSeenVersion != "" {
		t.Errorf("Settings.LastSeenVersion = %q, want empty for a document that never set it", reloaded.Settings.LastSeenVersion)
	}
}

func TestSetNexusProtocolRejectsInvalidFields(t *testing.T) {
	tooLong := strings.Repeat("a", maxNexusProtocolFieldLength+1)
	for _, test := range []struct {
		name     string
		snapshot NexusProtocolSnapshot
	}{
		{name: "NUL in command", snapshot: NexusProtocolSnapshot{Command: "path\x00.exe"}},
		{name: "NUL in icon", snapshot: NexusProtocolSnapshot{Icon: "icon\x00.ico"}},
		{name: "NUL in description", snapshot: NexusProtocolSnapshot{Description: "desc\x00"}},
		{name: "command too long", snapshot: NexusProtocolSnapshot{Command: tooLong}},
		{name: "icon too long", snapshot: NexusProtocolSnapshot{Icon: tooLong}},
		{name: "description too long", snapshot: NexusProtocolSnapshot{Description: tooLong}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			kept := NexusProtocolSnapshot{Command: "keep"}
			doc := Document{Settings: Settings{NexusProtocol: kept}}

			// Act
			err := doc.SetNexusProtocol(test.snapshot)

			// Assert
			if err == nil {
				t.Fatal("SetNexusProtocol() succeeded, want an error")
			}
			if doc.Settings.NexusProtocol != kept {
				t.Errorf("Settings.NexusProtocol was mutated on reject: %#v", doc.Settings.NexusProtocol)
			}
		})
	}
}

func TestSetNexusProtocolAcceptsAValidSnapshot(t *testing.T) {
	// Arrange
	var doc Document
	snapshot := NexusProtocolSnapshot{
		Command:     `"C:\Program Files\BentoMod\BentoMod.exe" "%1"`,
		Icon:        `C:\Program Files\BentoMod\BentoMod.exe,0`,
		Description: "URL:Nexus Mods Protocol",
	}

	// Act
	err := doc.SetNexusProtocol(snapshot)

	// Assert
	if err != nil {
		t.Fatalf("SetNexusProtocol() = %v, want no error", err)
	}
	if doc.Settings.NexusProtocol != snapshot {
		t.Errorf("Settings.NexusProtocol = %#v, want %#v", doc.Settings.NexusProtocol, snapshot)
	}
}

func TestSetNexusProtocolAcceptsTheZeroValueToClear(t *testing.T) {
	// Arrange
	doc := Document{Settings: Settings{NexusProtocol: NexusProtocolSnapshot{Command: "keep"}}}

	// Act
	err := doc.SetNexusProtocol(NexusProtocolSnapshot{})

	// Assert
	if err != nil {
		t.Fatalf("SetNexusProtocol() = %v, want no error", err)
	}
	if doc.Settings.NexusProtocol != (NexusProtocolSnapshot{}) {
		t.Errorf("Settings.NexusProtocol = %#v, want the zero value", doc.Settings.NexusProtocol)
	}
}

func TestNexusProtocolSurvivesASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	snapshot := NexusProtocolSnapshot{
		Command:     `"C:\Program Files\BentoMod\BentoMod.exe" "%1"`,
		Icon:        `C:\Program Files\BentoMod\BentoMod.exe,0`,
		Description: "URL:Nexus Mods Protocol",
	}
	if err := doc.SetNexusProtocol(snapshot); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if reloaded.Settings.NexusProtocol != snapshot {
		t.Errorf("Settings.NexusProtocol = %#v, want %#v", reloaded.Settings.NexusProtocol, snapshot)
	}
}

func TestDocumentWithNoNexusProtocolLoadsAsEmpty(t *testing.T) {
	// Arrange: a schema-1 document written before this field existed has no
	// "nexusProtocol" key. CurrentSchemaVersion stays 1; this is a pure
	// field addition with a zero default, not a migration.
	path := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion": 1, "settings": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	// Act
	reloaded, recovery := store.Load()

	// Assert
	if recovery.Recovered {
		t.Fatalf("Recovery = %#v, want Recovered = false for a schema-1 document", recovery)
	}
	if reloaded.Settings.NexusProtocol != (NexusProtocolSnapshot{}) {
		t.Errorf("Settings.NexusProtocol = %#v, want the zero value for a document that never set it", reloaded.Settings.NexusProtocol)
	}
}
