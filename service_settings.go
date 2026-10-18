package main

import (
	"github.com/Kuusouu/Cratebug/internal/metadata"
)

// MetadataState is the persisted document plus whether it had to be
// recovered from a damaged file, so the frontend can surface that as
// actionable feedback instead of silently losing track of the event.
type MetadataState struct {
	Document       metadata.Document `json:"document"`
	Recovered      bool              `json:"recovered"`
	RecoveryReason string            `json:"recoveryReason,omitempty"`
}

// Persists appearance and library preferences and manages the tag catalog and
// its per-mod assignments.
type SettingsService struct {
	*core
}

func newSettingsService(c *core) *SettingsService {
	return &SettingsService{core: c}
}

// Returns the persisted settings, tag catalog, and per-mod tag assignments.
func (s *SettingsService) LoadMetadata() MetadataState {
	doc, recovery := s.meta.load()
	state := MetadataState{Document: doc, Recovered: recovery.Recovered}
	if recovery.Cause != nil {
		state.RecoveryReason = recovery.Cause.Error()
	}
	return state
}

// Persists which store provider library auto-detection targets.
func (s *SettingsService) SetLibraryProvider(provider string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.SetLibraryProvider(provider)
	})
}

// Persists the appearance theme (system, light, or dark).
func (s *SettingsService) SetTheme(theme string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.SetTheme(theme)
	})
}

// Persists the view mode Cratebug opens to on the next launch.
func (s *SettingsService) SetDefaultViewMode(mode string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.SetDefaultViewMode(mode)
	})
}

// Persists the accent choice: a preset id, a custom hex color, or empty for
// the default preset.
func (s *SettingsService) SetAccentColor(color string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.SetAccentColor(color)
	})
}

// Persists whether destructive confirmation dialogs skip their countdown.
func (s *SettingsService) SetSkipDestructiveDelay(skip bool) error {
	return s.meta.update(func(doc *metadata.Document) error {
		doc.SetSkipDestructiveDelay(skip)
		return nil
	})
}

// Adds a new tag to the persisted catalog.
func (s *SettingsService) CreateTag(name string) (metadata.Tag, error) {
	var tag metadata.Tag
	err := s.meta.update(func(doc *metadata.Document) error {
		created, err := doc.CreateTag(name)
		tag = created
		return err
	})
	if err != nil {
		return metadata.Tag{}, err
	}
	return tag, nil
}

// Renames an existing tag in the persisted catalog.
func (s *SettingsService) RenameTag(id, name string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.RenameTag(id, name)
	})
}

// Removes a tag from the catalog and every mod it was assigned to.
func (s *SettingsService) DeleteTag(id string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		return doc.DeleteTag(id)
	})
}

// Assigns an existing catalog tag to one current scanner entry, establishing
// the entry's persistent identity first if this is the first metadata
// recorded for it.
func (s *SettingsService) AssignModTag(entryID, tagID string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		modID, err := doc.EnsureMod(entryID)
		if err != nil {
			return err
		}
		return doc.AssignTag(modID, tagID)
	})
}

// Removes a tag assignment from one current scanner entry.
func (s *SettingsService) UnassignModTag(entryID, tagID string) error {
	return s.meta.update(func(doc *metadata.Document) error {
		modID, err := doc.EnsureMod(entryID)
		if err != nil {
			return err
		}
		return doc.UnassignTag(modID, tagID)
	})
}
