package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/mutation"
)

func TestCreateTagRejectsCaseInsensitiveDuplicateNames(t *testing.T) {
	// Arrange
	var doc Document
	if _, err := doc.CreateTag("Combat"); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := doc.CreateTag("combat")

	// Assert
	if err == nil {
		t.Fatal("CreateTag() succeeded, want a duplicate-name error")
	}
}

func TestCreateTagRejectsEmptyName(t *testing.T) {
	// Arrange
	var doc Document

	// Act
	_, err := doc.CreateTag("   ")

	// Assert
	if err == nil {
		t.Fatal("CreateTag() succeeded, want an empty-name error")
	}
}

func TestRenameTagRejectsCollidingWithAnotherTag(t *testing.T) {
	// Arrange
	var doc Document
	if _, err := doc.CreateTag("Combat"); err != nil {
		t.Fatal(err)
	}
	skins, err := doc.CreateTag("Skins")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = doc.RenameTag(skins.ID, "combat")

	// Assert
	if err == nil {
		t.Fatal("RenameTag() succeeded, want a duplicate-name error")
	}
}

func TestRenameTagAllowsRenamingToItsOwnCurrentName(t *testing.T) {
	// Arrange
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = doc.RenameTag(tag.ID, "Combat")

	// Assert
	if err != nil {
		t.Fatalf("RenameTag() = %v, want no error renaming to the same name", err)
	}
}

func TestDeleteTagRemovesItFromCatalogAndAssignedMods(t *testing.T) {
	// Arrange
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := doc.DeleteTag(tag.ID); err != nil {
		t.Fatal(err)
	}

	// Assert
	if _, err := doc.tagIndex(tag.ID); err == nil {
		t.Error("deleted tag is still present in the catalog")
	}
	if containsString(doc.Mods[modID].Tags, tag.ID) {
		t.Error("deleted tag is still assigned to the mod")
	}
}

func TestAssignTagIsIdempotent(t *testing.T) {
	// Arrange
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}

	// Assert
	if got := len(doc.Mods[modID].Tags); got != 1 {
		t.Errorf("assigned tag count = %d, want 1", got)
	}
}

func TestAssignTagRejectsUnknownTagOrMod(t *testing.T) {
	// Arrange
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}

	// Act & Assert
	if err := doc.AssignTag(modID, "tag-unknown"); err == nil {
		t.Error("AssignTag() with an unknown tag succeeded, want an error")
	}
	if err := doc.AssignTag("mod-unknown", tag.ID); err == nil {
		t.Error("AssignTag() with an untracked mod succeeded, want an error")
	}
}

func TestUnassignTagWithoutExistingAssignmentIsANoOp(t *testing.T) {
	// Arrange
	var doc Document
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = doc.UnassignTag(modID, "tag-never-assigned")

	// Assert
	if err != nil {
		t.Fatalf("UnassignTag() = %v, want no error for an unassigned tag", err)
	}
}

func TestTagCatalogAndAssignmentSurviveASaveLoadRoundTrip(t *testing.T) {
	// Arrange
	store := NewStore(filepath.Join(t.TempDir(), "metadata.json"))
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod("mod:folder:example")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Load()

	// Assert
	if len(reloaded.Tags) != 1 || reloaded.Tags[0] != tag {
		t.Errorf("reloaded tag catalog = %#v, want [%#v]", reloaded.Tags, tag)
	}
	if !containsString(reloaded.Mods[modID].Tags, tag.ID) {
		t.Errorf("reloaded mod record lost its tag assignment: %#v", reloaded.Mods[modID])
	}
}

// Confirms a tag assignment stays attached to the same mod after a real
// rename and a real move, following the identity that ReconcileMod repoints.
func TestTagAssignmentFollowsAModThroughRenameAndMove(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "Example_9999999_P.pak"))
	entryID := scanEntryID(t, root, "Example_9999999_P.pak")
	var doc Document
	tag, err := doc.CreateTag("Combat")
	if err != nil {
		t.Fatal(err)
	}
	modID, err := doc.EnsureMod(entryID)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AssignTag(modID, tag.ID); err != nil {
		t.Fatal(err)
	}
	executor := mutation.NewExecutor(staticGameRunningChecker{})

	// Act
	renameResult, err := executor.Execute(mutation.NewRenameModOperation(root, entryID, "Renamed"))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ReconcileMod(renameResult.PreviousID, renameResult.ID) {
		t.Fatal("ReconcileMod() after rename = false, want true")
	}

	if err := os.Mkdir(filepath.Join(root, "Destination"), 0o700); err != nil {
		t.Fatal(err)
	}
	moveResult, err := executor.Execute(mutation.NewMoveModOperation(root, renameResult.ID, "Destination"))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ReconcileMod(moveResult.PreviousID, moveResult.ID) {
		t.Fatal("ReconcileMod() after move = false, want true")
	}

	// Assert
	if !containsString(doc.Mods[modID].Tags, tag.ID) {
		t.Errorf("mod %q lost its tag assignment after rename and move: %#v", modID, doc.Mods[modID])
	}
}
