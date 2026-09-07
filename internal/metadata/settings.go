package metadata

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Kuusouu/Cratebug/internal/gamedetect"
)

// A registry command is an executable path plus `"%1"`, not a novel.
// Anything longer is almost certainly junk pasted through a Wails binding.
const maxNexusProtocolFieldLength = 4096

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

// Sets the store provider library auto-detection targets, rejecting any
// value outside the set of providers this build registers. An empty string
// clears the override and restores the default provider.
func (doc *Document) SetLibraryProvider(provider string) error {
	if provider != "" && !gamedetect.ValidProvider(provider) {
		return fmt.Errorf("unsupported library provider: %q", provider)
	}
	doc.Settings.LibraryProvider = provider
	return nil
}

// Records that the post-update "what's new" notice has been shown for
// version. Unlike the settings above, this isn't Wails-bound as a directly
// callable setter, so it has no untrusted input to validate.
func (doc *Document) SetLastSeenVersion(version string) {
	doc.Settings.LastSeenVersion = version
}

// Records the previous nxm:// handler so unregister can restore it. The
// snapshot is paths and display text, not a secret. The zero value clears
// it. Wails bindings will later be callable from devtools, so each field
// is rejected if it contains a NUL or exceeds a registry-command length.
func (doc *Document) SetNexusProtocol(snapshot NexusProtocolSnapshot) error {
	if err := validateNexusProtocolField("command", snapshot.Command); err != nil {
		return err
	}
	if err := validateNexusProtocolField("icon", snapshot.Icon); err != nil {
		return err
	}
	if err := validateNexusProtocolField("description", snapshot.Description); err != nil {
		return err
	}

	doc.Settings.NexusProtocol = snapshot
	return nil
}

// Records that the user turned the nxm:// handler off. Missing (including
// every document written before this field existed) means the handler
// defaults on, so startup may register silently when nothing owns the scheme.
func (doc *Document) SetNexusProtocolOptOut(optOut bool) {
	doc.Settings.NexusProtocolOptOut = optOut
}

func validateNexusProtocolField(name, value string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("nexus protocol %s contains a NUL byte", name)
	}
	if len(value) > maxNexusProtocolFieldLength {
		return fmt.Errorf("nexus protocol %s exceeds %d bytes", name, maxNexusProtocolFieldLength)
	}
	return nil
}
