package conflict

import (
	"sort"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// Distinguishes why an overlap needs attention. SPEC.md's "Asset conflict"
// term covers both: what makes SamePriority the case that actually needs a
// user decision is that the involved mods have no defined load order for
// the paths they share (SPEC.md also calls this "duplicate priority",
// contrasting it with the broader asset-conflict idea). CrossPriority mods
// already load deterministically by priority, so the overlap is
// informational rather than something that needs resolving.
type Relationship string

const (
	SamePriority  Relationship = "same_priority"
	CrossPriority Relationship = "cross_priority"
)

// One enabled mod's role within a Group: its identity, current priority,
// and the specific internal paths it shares with at least one other mod in
// the group. OverlappingPaths is a subset of the mod's full internal
// listing, not the whole thing.
type Participant struct {
	EntryID          string             `json:"entryID"`
	DisplayName      string             `json:"displayName"`
	Priority         discovery.Priority `json:"priority"`
	OverlappingPaths []string           `json:"overlappingPaths"`
}

// A maximal cluster of enabled mods that transitively share at least one
// internal asset path. PathCount is the number of distinct paths shared
// somewhere within the group, not a sum across participants.
type Group struct {
	Participants []Participant `json:"participants"`
	Relationship Relationship  `json:"relationship"`
	PathCount    int           `json:"pathCount"`
}

// The complete result of one conflict scan.
type Result struct {
	Groups []Group `json:"groups"`
	// Entry IDs of enabled mods whose internal paths could not be resolved
	// (an encrypted or otherwise undeterminable bundle, or a UAssetToolRivals
	// failure). These mods are excluded from every Group rather than
	// silently treated as conflict-free, since that would be a wrong answer
	// dressed up as a clean one.
	Unavailable []string `json:"unavailable"`
}

// Detect finds internal asset-path overlaps among entries' enabled mods.
// paths supplies each entry's already-resolved internal path listing, keyed
// by discovery.Entry.ID; Detect performs no UAssetToolRivals calls of its
// own; see internal/modtype's SessionClassifier.Paths and ListInternalPaths
// for how callers resolve entries into this shape, reusing whatever a prior
// classification pass already fetched. An entry present in entries but
// absent from paths is reported in Result.Unavailable instead of being
// treated as having no content.
//
// Disabled mods and non-mod entries (orphaned sidecars) never appear in the
// result, matching the roadmap's requirement that conflict detection only
// considers enabled mods.
func Detect(entries []discovery.Entry, paths map[string][]string) Result {
	enabled := make(map[string]discovery.Entry)
	for _, entry := range entries {
		if entry.Kind == discovery.EntryMod && entry.State == discovery.StateEnabled {
			enabled[entry.ID] = entry
		}
	}

	pathIndex := make(map[string][]string) // internal asset path -> entry IDs that contain it
	var unavailable []string
	for id := range enabled {
		list, ok := paths[id]
		if !ok {
			unavailable = append(unavailable, id)
			continue
		}
		for _, p := range list {
			pathIndex[p] = append(pathIndex[p], id)
		}
	}
	sort.Strings(unavailable)

	groups := make([]Group, 0, len(pathIndex))
	for _, memberIDs := range connectedEntries(pathIndex) {
		groups = append(groups, buildGroup(memberIDs, enabled, paths, pathIndex))
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Participants[0].EntryID < groups[j].Participants[0].EntryID
	})

	return Result{Groups: groups, Unavailable: unavailable}
}

// Finds every maximal set of entry IDs transitively connected by sharing at
// least one path (a path index entry with 2+ contributing entries is one
// edge). Entries that share nothing with anyone never appear in the result.
func connectedEntries(pathIndex map[string][]string) [][]string {
	parent := make(map[string]string)
	var find func(string) string
	find = func(x string) string {
		if _, ok := parent[x]; !ok {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for _, ids := range pathIndex {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids[1:] {
			union(ids[0], id)
		}
	}

	members := make(map[string]map[string]struct{})
	for _, ids := range pathIndex {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			root := find(id)
			if members[root] == nil {
				members[root] = make(map[string]struct{})
			}
			members[root][id] = struct{}{}
		}
	}

	components := make([][]string, 0, len(members))
	for _, set := range members {
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		components = append(components, ids)
	}
	return components
}

// Builds one Group from a connected component, deriving each participant's
// shared paths and the group's overall priority relationship.
func buildGroup(memberIDs []string, enabled map[string]discovery.Entry, paths map[string][]string, pathIndex map[string][]string) Group {
	distinctPaths := make(map[string]struct{})
	participants := make([]Participant, 0, len(memberIDs))
	relationship := SamePriority

	for i, id := range memberIDs {
		entry := enabled[id]

		var overlapping []string
		for _, p := range paths[id] {
			if len(pathIndex[p]) >= 2 {
				overlapping = append(overlapping, p)
				distinctPaths[p] = struct{}{}
			}
		}
		sort.Strings(overlapping)

		participants = append(participants, Participant{
			EntryID:          id,
			DisplayName:      entry.DisplayName,
			Priority:         entry.Priority,
			OverlappingPaths: overlapping,
		})

		if i > 0 && !samePriorityTier(entry.Priority, enabled[memberIDs[0]].Priority) {
			relationship = CrossPriority
		}
	}

	return Group{
		Participants: participants,
		Relationship: relationship,
		PathCount:    len(distinctPaths),
	}
}

// Reports whether two priorities represent the same load-order tier. Only
// Kind and Value matter: Raw and TrailingNines vary per mod (they include
// the filename stem) and are irrelevant to tier comparison. Comparing Kind
// alongside Value, rather than Value alone, matters because
// discovery.Priority leaves Value at its zero value for both the
// leading-bang form and an entry with no recognized priority markup at
// all — comparing Value alone would wrongly treat an explicit maximum
// priority ("!") as equal to a mod that was never given a priority.
func samePriorityTier(a, b discovery.Priority) bool {
	return a.Kind == b.Kind && a.Value == b.Value
}
