package metadata

import (
	"fmt"
	"regexp"
)

var validThemes = map[string]bool{
	"system": true,
	"light":  true,
	"dark":   true,
}

var validViewModes = map[string]bool{
	"compact": true,
	"large":   true,
	"list":    true,
}

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Sets the persisted appearance theme. Rejects any value outside the fixed
// set the frontend actually offers, since Wails bindings are callable from
// devtools, not just the intended UI.
func (doc *Document) SetTheme(theme string) error {
	if !validThemes[theme] {
		return fmt.Errorf("unsupported theme: %q", theme)
	}
	doc.Settings.Theme = theme
	return nil
}

// Sets the view mode Cratebug opens to on the next launch.
func (doc *Document) SetDefaultViewMode(mode string) error {
	if !validViewModes[mode] {
		return fmt.Errorf("unsupported view mode: %q", mode)
	}
	doc.Settings.DefaultViewMode = mode
	return nil
}

// Sets the accent color override, a 6-digit hex string like "#f0a54d". An
// empty string clears the override and restores the active theme's own
// default accent, so this doubles as the reset path.
func (doc *Document) SetAccentColor(color string) error {
	if color != "" && !hexColorPattern.MatchString(color) {
		return fmt.Errorf("accent color must be a 6-digit hex value like #f0a54d, got %q", color)
	}
	doc.Settings.AccentColor = color
	return nil
}

// Records that the post-update "what's new" notice has been shown for
// version. Unlike the settings above, this isn't Wails-bound as a directly
// callable setter, so it has no untrusted input to validate.
func (doc *Document) SetLastSeenVersion(version string) {
	doc.Settings.LastSeenVersion = version
}
