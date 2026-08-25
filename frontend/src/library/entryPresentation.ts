import type { discovery, modtype } from "../../wailsjs/go/models";

// Returns the category name when classified, or empty when not yet available.
export function entryCategoryLabel(identity?: modtype.Identity | null): string {
	if (!identity?.category) {
		return "";
	}
	return identity.category;
}

// Converts category name to CSS-friendly class name slug.
export function categorySlug(category?: string | null): string {
	if (!category) return "";
	return category.toLowerCase().replace(/\s+/g, "-");
}

// Formats hero and optional skin name into a human-readable label.
export function entryCharacterLabel(identity?: modtype.Identity | null): string {
	if (!identity?.characterName) {
		return "";
	}
	if (identity.skinName) {
		return `${identity.characterName} (${identity.skinName})`;
	}
	return identity.characterName;
}

// Keeps state wording consistent wherever a discovered entry is presented.
export function entryStateLabel(entry: discovery.Entry): string {
	if (entry.kind === "orphaned_sidecar") return "Orphaned sidecar";

	return entry.state === "disabled" ? "Disabled" : "Enabled";
}

// Same-stem primaries make it unclear which file a mutation should act on.
export function hasAmbiguousPrimary(entry: discovery.Entry): boolean {
	return entry.issues?.some((issue) => issue.code === "ambiguous-primary") ?? false;
}

// An IoStore bundle missing its .utoc or .ucas cannot be safely renamed or moved.
export function hasMissingSidecar(entry: discovery.Entry): boolean {
	return (
		entry.issues?.some(
			(issue) => issue.code === "missing-utoc" || issue.code === "missing-ucas",
		) ?? false
	);
}

// Shared by Enable/Disable and any control that mutates the primary in place.
export function canChangeModState(entry: discovery.Entry): boolean {
	return entry.kind === "mod" && entry.primaryPath !== undefined && !hasAmbiguousPrimary(entry);
}

// Rename, priority, and move all require the same complete, unambiguous bundle.
export function canOrganizeMod(entry: discovery.Entry): boolean {
	return canChangeModState(entry) && !hasMissingSidecar(entry);
}

// Deletion permits incomplete IoStore bundles, unlike rename, priority, and
// move, because it only sends whichever recognized members are present.
export function canDeleteMod(entry: discovery.Entry): boolean {
	return canChangeModState(entry);
}

// Tags attach to a mod's persistent identity independent of its current
// filename or bundle completeness, so any scanned mod can be tagged even if
// its file state blocks rename, move, or delete.
export function canTagMod(entry: discovery.Entry): boolean {
	return entry.kind === "mod";
}
