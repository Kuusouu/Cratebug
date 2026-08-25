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

let heroPortraitModules: Record<string, string> = {};
try {
	heroPortraitModules = import.meta.glob<string>("../assets/heroes/*.png", {
		eager: true,
		import: "default",
	});
} catch {
	// Bun's test runner does not support import.meta.glob
}

const heroPortraitsByID: Record<string, string> = {};
for (const [path, url] of Object.entries(heroPortraitModules)) {
	const match = path.match(/(\d{4,7})\.png$/i);
	if (match?.[1] && url) {
		heroPortraitsByID[match[1]] = url;
	}
}

// Resolves portrait URL from a map of character/skin ID -> URL,
// preferring a skin-specific portrait when available and falling back to the hero avatar.
export function resolveHeroPortraitUrl(
	portraitsByID: Record<string, string>,
	identity?: modtype.Identity | null,
): string | null {
	if (!identity) {
		return null;
	}
	if (identity.skinID && portraitsByID[identity.skinID]) {
		return portraitsByID[identity.skinID] ?? null;
	}
	if (identity.characterID && portraitsByID[identity.characterID]) {
		return portraitsByID[identity.characterID] ?? null;
	}
	return null;
}

// Returns the bundled hero or skin portrait image URL if available, or null.
export function entryHeroPortraitUrl(identity?: modtype.Identity | null): string | null {
	return resolveHeroPortraitUrl(heroPortraitsByID, identity);
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
