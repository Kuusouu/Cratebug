import type { discovery } from "../../wailsjs/go/models";

export type LibraryState = "initial" | "loading" | "populated" | "empty" | "error";

// A drag can carry either a folder (dragged from the sidebar, drop target
// must not be itself or a descendant) or a mod (dragged from the catalog,
// any folder is a valid destination). Shared between LibraryScreen (which
// owns the drag), FolderNavigation (the drop target), and ModCatalog (a
// second drag source) since the source and target live in sibling components.
export type DraggedItem =
	| { type: "folder"; path: string }
	| { type: "mod"; entry: discovery.Entry };

// A dragged folder cannot be dropped into itself or its own descendant; a
// dragged mod has no such constraint, any folder is a valid destination.
// Shared by FolderNavigation (deciding whether to show drag-over highlight
// and allow the drop) and LibraryScreen (deciding whether to execute the
// move once dropped) so the two can't drift out of sync.
export function isValidDropTarget(draggedItem: DraggedItem | null, targetFolder: string): boolean {
	if (!draggedItem) return false;
	if (draggedItem.type === "mod") return true;
	return draggedItem.path !== targetFolder && !targetFolder.startsWith(`${draggedItem.path}/`);
}

export const viewModes = ["compact", "large", "list"] as const;

export type ViewMode = (typeof viewModes)[number];

export const viewModeLabels: Record<ViewMode, string> = {
	compact: "Compact",
	large: "Large",
	list: "List",
};

export const themes = ["system", "light", "dark"] as const;

export type Theme = (typeof themes)[number];

export const themeLabels: Record<Theme, string> = {
	system: "System",
	light: "Light",
	dark: "Dark",
};

// Store providers library auto-detection can target, matching the providers
// the Go side registers. Epic Games joins when its provider ships; the
// Settings selector shows it as unavailable until then.
export const libraryProviders = ["steam"] as const;

export type LibraryProvider = (typeof libraryProviders)[number];

export const libraryProviderLabels: Record<LibraryProvider, string> = {
	steam: "Steam",
};
