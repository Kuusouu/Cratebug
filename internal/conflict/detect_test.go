package conflict

import (
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

func enabledMod(id string, priority discovery.Priority) discovery.Entry {
	return discovery.Entry{
		ID:          id,
		Kind:        discovery.EntryMod,
		State:       discovery.StateEnabled,
		DisplayName: id,
		Priority:    priority,
	}
}

func disabledMod(id string, priority discovery.Priority) discovery.Entry {
	entry := enabledMod(id, priority)
	entry.State = discovery.StateDisabled
	return entry
}

func noPriority() discovery.Priority {
	return discovery.Priority{Kind: discovery.PriorityNone}
}

func bangPriority() discovery.Priority {
	return discovery.Priority{Kind: discovery.PriorityLeadingBang}
}

func trailingNinePriority(value int) discovery.Priority {
	return discovery.Priority{Kind: discovery.PriorityTrailingNine, Value: value}
}

func findGroup(t *testing.T, result Result, participantID string) Group {
	t.Helper()
	for _, group := range result.Groups {
		for _, p := range group.Participants {
			if p.EntryID == participantID {
				return group
			}
		}
	}
	t.Fatalf("no group contains participant %q; groups = %+v", participantID, result.Groups)
	return Group{}
}

func TestDetectGroupsTwoModsSharingOnePathAtSamePriority(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()),
	}
	paths := map[string][]string{
		"mod-a": {"Marvel/Content/Characters/1044/SK_A.uasset"},
		"mod-b": {"Marvel/Content/Characters/1044/SK_A.uasset"},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	group := result.Groups[0]
	if group.Relationship != SamePriority {
		t.Errorf("Relationship = %q, want %q", group.Relationship, SamePriority)
	}
	if group.PathCount != 1 {
		t.Errorf("PathCount = %d, want 1", group.PathCount)
	}
	if len(group.Participants) != 2 {
		t.Fatalf("len(Participants) = %d, want 2", len(group.Participants))
	}
	for _, p := range group.Participants {
		if len(p.OverlappingPaths) != 1 || p.OverlappingPaths[0] != "Marvel/Content/Characters/1044/SK_A.uasset" {
			t.Errorf("participant %q OverlappingPaths = %v, want the one shared path", p.EntryID, p.OverlappingPaths)
		}
	}
}

func TestDetectMarksDifferentTrailingNineValuesAsCrossPriority(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", trailingNinePriority(1)),
		enabledMod("mod-b", trailingNinePriority(2)),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	if result.Groups[0].Relationship != CrossPriority {
		t.Errorf("Relationship = %q, want %q", result.Groups[0].Relationship, CrossPriority)
	}
}

// A leading-bang mod and an unprioritized mod both leave Priority.Value at
// its zero value, so comparing Value alone would wrongly call them the same
// tier. Kind must be compared too.
func TestDetectDoesNotConfuseLeadingBangWithNoPriority(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", bangPriority()),
		enabledMod("mod-b", noPriority()),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	if result.Groups[0].Relationship != CrossPriority {
		t.Errorf("Relationship = %q, want %q (leading-bang and no-priority are different tiers)", result.Groups[0].Relationship, CrossPriority)
	}
}

func TestDetectTreatsTwoLeadingBangModsAsSamePriority(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", bangPriority()),
		enabledMod("mod-b", bangPriority()),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 || result.Groups[0].Relationship != SamePriority {
		t.Errorf("Groups = %+v, want one SamePriority group", result.Groups)
	}
}

func TestDetectExcludesDisabledModsEvenWhenPathsOverlap(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		disabledMod("mod-b", noPriority()),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none (only one enabled mod touches the shared path)", result.Groups)
	}
}

func TestDetectIgnoresOrphanedSidecarEntries(t *testing.T) {
	// Arrange
	orphan := discovery.Entry{ID: "orphan-1", Kind: discovery.EntryOrphanedSidecar, State: discovery.StateEnabled}
	entries := []discovery.Entry{orphan}
	paths := map[string][]string{"orphan-1": {"whatever.uasset"}}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 0 || len(result.Unavailable) != 0 {
		t.Errorf("result = %+v, want an empty result for a non-mod entry", result)
	}
}

func TestDetectReportsUnavailableForEntryMissingFromPaths(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()), // encrypted / undeterminable: no paths entry
	}
	paths := map[string][]string{
		"mod-a": {"Marvel/Content/Characters/1044/SK_A.uasset"},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none", result.Groups)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != "mod-b" {
		t.Errorf("Unavailable = %v, want [mod-b]", result.Unavailable)
	}
}

func TestDetectDoesNotLetAnUnavailableModHideARealOverlap(t *testing.T) {
	// Arrange: mod-c is unavailable, but mod-a and mod-b still overlap directly.
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()),
		enabledMod("mod-c", noPriority()),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 {
		t.Fatalf("Groups = %+v, want exactly one group for mod-a/mod-b", result.Groups)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != "mod-c" {
		t.Errorf("Unavailable = %v, want [mod-c]", result.Unavailable)
	}
}

func TestDetectFormsOneTransitiveGroupAcrossAChainOfMods(t *testing.T) {
	// Arrange: mod-a/mod-b share pathX, mod-b/mod-c share pathY. mod-a and
	// mod-c share nothing directly but belong in the same connected group.
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()),
		enabledMod("mod-c", noPriority()),
	}
	pathX := "Marvel/Content/Characters/1044/SK_X.uasset"
	pathY := "Marvel/Content/Characters/1044/SK_Y.uasset"
	paths := map[string][]string{
		"mod-a": {pathX},
		"mod-b": {pathX, pathY},
		"mod-c": {pathY},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 1 {
		t.Fatalf("Groups = %+v, want one transitively-connected group", result.Groups)
	}
	group := result.Groups[0]
	if len(group.Participants) != 3 {
		t.Fatalf("Participants = %+v, want all three mods", group.Participants)
	}
	if group.PathCount != 2 {
		t.Errorf("PathCount = %d, want 2 (pathX and pathY)", group.PathCount)
	}

	byID := make(map[string]Participant, len(group.Participants))
	for _, p := range group.Participants {
		byID[p.EntryID] = p
	}
	if got := byID["mod-a"].OverlappingPaths; len(got) != 1 || got[0] != pathX {
		t.Errorf("mod-a OverlappingPaths = %v, want [%s]", got, pathX)
	}
	if got := byID["mod-c"].OverlappingPaths; len(got) != 1 || got[0] != pathY {
		t.Errorf("mod-c OverlappingPaths = %v, want [%s]", got, pathY)
	}
	if got := byID["mod-b"].OverlappingPaths; len(got) != 2 {
		t.Errorf("mod-b OverlappingPaths = %v, want both paths", got)
	}
}

func TestDetectReturnsNoGroupsWhenNothingOverlaps(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()),
	}
	paths := map[string][]string{
		"mod-a": {"Marvel/Content/Characters/1044/SK_A.uasset"},
		"mod-b": {"Marvel/Content/Characters/1020/SK_B.uasset"},
	}

	// Act
	result := Detect(entries, paths)

	// Assert
	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none", result.Groups)
	}
	if len(result.Unavailable) != 0 {
		t.Errorf("Unavailable = %v, want none", result.Unavailable)
	}
}

func TestDetectFindGroupHelperLocatesParticipant(t *testing.T) {
	// Arrange
	entries := []discovery.Entry{
		enabledMod("mod-a", noPriority()),
		enabledMod("mod-b", noPriority()),
	}
	sharedPath := "Marvel/Content/Characters/1044/SK_A.uasset"
	paths := map[string][]string{
		"mod-a": {sharedPath},
		"mod-b": {sharedPath},
	}

	// Act
	result := Detect(entries, paths)
	group := findGroup(t, result, "mod-a")

	// Assert
	if len(group.Participants) != 2 {
		t.Errorf("len(Participants) = %d, want 2", len(group.Participants))
	}
}
