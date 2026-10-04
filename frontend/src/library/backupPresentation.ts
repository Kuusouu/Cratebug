export type BackupCounts = {
	mods: number;
	iostore: number;
	classic: number;
	invalid: number;
};

export type BackupProgressView = {
	current: number;
	total: number;
	currentFile: string;
};

function pluralize(count: number, singular: string, plural: string): string {
	return `${count} ${count === 1 ? singular : plural}`;
}

function formatCountsLine(verbPast: string, counts: BackupCounts): string {
	return `${verbPast} ${pluralize(counts.mods, "mod", "mods")} (${counts.iostore} IoStore, ${counts.classic} Classic, ${counts.invalid} Invalid).`;
}

// Renders the backup report line. Only the mod noun pluralizes; the
// category labels stay fixed so the line matches the counts everywhere it
// appears.
export function formatBackupSummary(counts: BackupCounts): string {
	return formatCountsLine("Backed up", counts);
}

// Renders the restore report line, mirroring the backup summary.
export function formatRestoreSummary(counts: BackupCounts): string {
	return formatCountsLine("Restored", counts);
}

// Renders the staged contents for the restore confirm preview. "Holds"
// keeps the preview distinct from the completed-restore report line.
export function formatStagedCounts(counts: BackupCounts): string {
	return formatCountsLine("Backup holds", counts);
}

// Renders the backup file's date for the restore preview, labeled as the
// file's own date rather than a verified backup timestamp.
export function formatBackupFileDate(zipModified: string): string {
	const parsed = new Date(zipModified);
	if (Number.isNaN(parsed.getTime())) {
		return "Backup file date unknown.";
	}
	return `Backup file from ${parsed.toLocaleString()}.`;
}

// Describes what the restore preview found inside the archive: whether
// metadata rides along, and the honest consequence when it does not.
export function formatRestoreMetadataClause(metadataPresent: boolean): string {
	if (metadataPresent) {
		return "Includes Cratebug metadata; tags and settings will be restored with the library.";
	}
	return "No metadata in this backup; the current tags and settings will be kept.";
}
