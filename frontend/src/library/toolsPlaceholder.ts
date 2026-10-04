// Placeholder catalog for the Tools dialog. Every entry is dummy data rendered
// as-is; no entry here performs any action or reaches the Go backend. Real
// tool wiring belongs to future roadmap phases and must replace these
// placeholders rather than build on them.
export type PlaceholderToolID =
	| "character-data"
	| "backup-create"
	| "backup-restore"
	| "bento-import";

export type PlaceholderTool = {
	id: PlaceholderToolID;
	title: string;
	description: string;
	// Short human-readable status line shown under the description.
	status: string;
	actionLabel: string;
};

export const placeholderTools: readonly PlaceholderTool[] = [
	{
		id: "character-data",
		title: "Update character data",
		description: "Refresh the hero and skin name table used for mod classification.",
		status: "Character table v0 (placeholder) · Updated: never",
		actionLabel: "Check for update",
	},
	{
		id: "backup-create",
		title: "Back up library",
		description: "Snapshot the mod library and Cratebug metadata to a backup.",
		status: "Last backup: never (placeholder)",
		actionLabel: "Create backup",
	},
	{
		id: "backup-restore",
		title: "Restore backup",
		description: "Restore the library and metadata from a previous backup.",
		status: "No backups found (placeholder)",
		actionLabel: "Choose backup",
	},
	{
		id: "bento-import",
		title: "Import from BentoMod",
		description: "Preview and import selected BentoMod settings into Cratebug.",
		status: "BentoMod not detected (placeholder)",
		actionLabel: "Preview import",
	},
];
