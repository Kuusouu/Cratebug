import type { discovery, install } from "../../wailsjs/go/models";

export type ModConfig = {
	id: string;
	selected: boolean;
	modName: string;
	destinationFolder: string;
	overwrite: boolean;
};

// Builds the default configuration for a freshly discovered preview item: included,
// using its derived name and preview destination, not overwriting.
export function defaultModConfig(item: install.PreviewItem): ModConfig {
	return {
		id: item.id,
		selected: true,
		modName: item.modName,
		destinationFolder: item.destinationFolder,
		overwrite: false,
	};
}

// Converts an unknown Wails failure (typically an Error, but not guaranteed) into a
// displayable message.
export function formatWailsError(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}

// Formats raw byte counts into clean human-readable sizes.
export function formatBytes(bytes: number): string {
	if (bytes <= 0) return "0 B";
	const units = ["B", "KB", "MB", "GB"];
	const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
	const size = bytes / 1024 ** index;
	return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

// Validates a user-entered mod name against Windows filename rules.
export function validateInstallModName(name: string): string | null {
	if (name.trim() === "") {
		return "Name cannot be empty.";
	}
	if (name.endsWith(" ") || name.endsWith(".")) {
		return "Name cannot end with a space or period.";
	}
	if (/[<>:"/\\|?*]/.test(name) || [...name].some((char) => char.charCodeAt(0) < 32)) {
		return "Name contains a Windows-reserved character.";
	}
	return null;
}

// Extracts the base mod stem from a filename without extension or disabled suffixes.
export function extractModStem(fileName: string): string {
	const base = fileName.split("/").pop()?.split("\\").pop() ?? fileName;
	const lower = base.toLowerCase();
	for (const ext of [".pak_crateoff", ".bak_bento", ".pak_disabled", ".pak", ".utoc", ".ucas"]) {
		if (lower.endsWith(ext)) {
			return base.slice(0, base.length - ext.length);
		}
	}
	return base;
}

export type LiveCollision = {
	hasCollision: boolean;
	description?: string | undefined;
	existingModID?: string | undefined;
};

// Checks whether an item's current configuration collides with an existing library entry.
export function detectLibraryCollision(
	item: install.PreviewItem,
	config: ModConfig | undefined,
	libraryEntries: readonly discovery.Entry[] = [],
): LiveCollision {
	if (!config?.selected) {
		return { hasCollision: false };
	}

	const targetFolder = config.destinationFolder.toLowerCase().replace(/\\/g, "/");
	const targetName = config.modName.toLowerCase().trim();

	for (const entry of libraryEntries) {
		const entryFolder = (entry.relativeFolder || "").toLowerCase().replace(/\\/g, "/");
		if (entryFolder !== targetFolder) {
			continue;
		}

		const entryDisplayName = entry.displayName.toLowerCase().trim();
		const entryStem = entry.primaryPath ? extractModStem(entry.primaryPath).toLowerCase() : "";

		if (entryDisplayName === targetName || (entryStem && entryStem === targetName)) {
			const folderLabel = config.destinationFolder
				? `'${config.destinationFolder}'`
				: "the library root";
			return {
				hasCollision: true,
				existingModID: entry.id,
				description: `A mod named "${entry.displayName}" already exists in ${folderLabel}.`,
			};
		}
	}

	// If no match in live entries, check initial backend scan collision if destinationFolder and name were unchanged
	if (
		item.collision?.hasCollision &&
		(config.destinationFolder || "") === (item.destinationFolder || "") &&
		(config.modName || "") === (item.modName || "")
	) {
		return {
			hasCollision: true,
			existingModID: item.collision.existingModID,
			description: item.collision.description,
		};
	}

	return { hasCollision: false };
}

// Checks whether any staged mod has an unacknowledged file collision.
export function hasUnresolvedCollisions(
	items: readonly install.PreviewItem[],
	configs: Record<string, ModConfig>,
	libraryEntries: readonly discovery.Entry[] = [],
): boolean {
	return items.some((item) => {
		const config = configs[item.id];
		if (!config?.selected) return false;
		if (config.overwrite) return false;

		const liveCollision = detectLibraryCollision(item, config, libraryEntries);
		return liveCollision.hasCollision;
	});
}

// Reports whether any selected item carries an unresolved staging issue (e.g. a bundle
// that could not be fully copied), which must be excluded rather than installed silently.
export function hasBlockingIssues(
	items: readonly install.PreviewItem[],
	configs: Record<string, ModConfig>,
): boolean {
	return items.some((item) => {
		const config = configs[item.id];
		if (!config?.selected) return false;
		return (item.issues?.length ?? 0) > 0;
	});
}

// Detects whether multiple selected items in the same batch target the same destination folder and mod name.
export function findBatchCollisions(
	items: readonly install.PreviewItem[],
	configs: Record<string, ModConfig>,
): Record<string, string> {
	const seenDestinations = new Map<string, string[]>();
	const errors: Record<string, string> = {};

	for (const item of items) {
		const config = configs[item.id];
		if (!config?.selected) continue;

		const targetFolder = config.destinationFolder.toLowerCase().replace(/\\/g, "/");
		const targetName = config.modName.toLowerCase().trim();
		const key = `${targetFolder}::${targetName}`;

		const existing = seenDestinations.get(key);
		if (existing) {
			existing.push(item.id);
		} else {
			seenDestinations.set(key, [item.id]);
		}
	}

	for (const [, itemIDs] of seenDestinations) {
		if (itemIDs.length > 1) {
			for (const id of itemIDs) {
				errors[id] =
					"Multiple selected mods in this batch have the same destination folder and name.";
			}
		}
	}

	return errors;
}
