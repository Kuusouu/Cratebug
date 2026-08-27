package metadata

import (
	"path/filepath"
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
