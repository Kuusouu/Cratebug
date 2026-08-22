package metadata

import (
	"fmt"
	"strings"
)

const tagIDPrefix = "tag-"

// Tag is one entry in the persisted catalog that mods can be assigned to.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Adds a new tag to the catalog. Names are compared case-insensitively so
// "Combat" and "combat" cannot both exist as separate tags.
func (doc *Document) CreateTag(name string) (Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, fmt.Errorf("tag name cannot be empty")
	}
	if _, exists := doc.findTagByName(name); exists {
		return Tag{}, fmt.Errorf("tag %q already exists", name)
	}

	id, err := newID(tagIDPrefix)
	if err != nil {
		return Tag{}, err
	}

	tag := Tag{ID: id, Name: name}
	doc.Tags = append(doc.Tags, tag)
	return tag, nil
}

// Renames an existing catalog tag.
func (doc *Document) RenameTag(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	index, err := doc.tagIndex(id)
	if err != nil {
		return err
	}
	if existing, exists := doc.findTagByName(name); exists && existing.ID != id {
		return fmt.Errorf("tag %q already exists", name)
	}

	doc.Tags[index].Name = name
	return nil
}

// Removes a tag from the catalog and every mod record it was assigned to.
func (doc *Document) DeleteTag(id string) error {
	index, err := doc.tagIndex(id)
	if err != nil {
		return err
	}
	doc.Tags = append(doc.Tags[:index], doc.Tags[index+1:]...)

	for modID, record := range doc.Mods {
		record.Tags = removeString(record.Tags, id)
		doc.Mods[modID] = record
	}
	return nil
}

// Assigns an existing catalog tag to a mod's persistent identity. Assigning
// an already-assigned tag is a no-op rather than an error.
func (doc *Document) AssignTag(modID, tagID string) error {
	if _, err := doc.tagIndex(tagID); err != nil {
		return err
	}

	record, ok := doc.Mods[modID]
	if !ok {
		return fmt.Errorf("mod is not tracked: %q", modID)
	}
	if containsString(record.Tags, tagID) {
		return nil
	}

	record.Tags = append(record.Tags, tagID)
	doc.Mods[modID] = record
	return nil
}

// Removes a tag assignment from a mod's persistent identity. Removing a tag
// that was not assigned is a no-op rather than an error.
func (doc *Document) UnassignTag(modID, tagID string) error {
	record, ok := doc.Mods[modID]
	if !ok {
		return fmt.Errorf("mod is not tracked: %q", modID)
	}

	record.Tags = removeString(record.Tags, tagID)
	doc.Mods[modID] = record
	return nil
}

func (doc Document) findTagByName(name string) (Tag, bool) {
	for _, tag := range doc.Tags {
		if strings.EqualFold(tag.Name, name) {
			return tag, true
		}
	}
	return Tag{}, false
}

func (doc Document) tagIndex(id string) (int, error) {
	for index, tag := range doc.Tags {
		if tag.ID == id {
			return index, nil
		}
	}
	return -1, fmt.Errorf("tag is not present in the catalog: %q", id)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	var filtered []string
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
