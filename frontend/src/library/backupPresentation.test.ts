import { describe, expect, test } from "bun:test";
import {
	formatBackupFileDate,
	formatBackupSummary,
	formatRestoreMetadataClause,
	formatRestoreSummary,
	formatStagedCounts,
} from "./backupPresentation";

describe("formatBackupSummary", () => {
	test("reports every category in the fixed backup line", () => {
		expect(formatBackupSummary({ mods: 12, iostore: 8, classic: 3, invalid: 1 })).toBe(
			"Backed up 12 mods (8 IoStore, 3 Classic, 1 Invalid).",
		);
	});

	test("uses the singular mod noun for a single-mod backup", () => {
		expect(formatBackupSummary({ mods: 1, iostore: 1, classic: 0, invalid: 0 })).toBe(
			"Backed up 1 mod (1 IoStore, 0 Classic, 0 Invalid).",
		);
	});

	test("reports an empty library without special-casing", () => {
		expect(formatBackupSummary({ mods: 0, iostore: 0, classic: 0, invalid: 0 })).toBe(
			"Backed up 0 mods (0 IoStore, 0 Classic, 0 Invalid).",
		);
	});
});

describe("formatRestoreSummary", () => {
	test("mirrors the backup line with the restore verb", () => {
		expect(formatRestoreSummary({ mods: 5, iostore: 2, classic: 2, invalid: 1 })).toBe(
			"Restored 5 mods (2 IoStore, 2 Classic, 1 Invalid).",
		);
	});
});

describe("formatStagedCounts", () => {
	test("describes staged contents without claiming a restore", () => {
		expect(formatStagedCounts({ mods: 5, iostore: 2, classic: 2, invalid: 1 })).toBe(
			"Backup holds 5 mods (2 IoStore, 2 Classic, 1 Invalid).",
		);
	});
});

describe("formatBackupFileDate", () => {
	test("labels a parseable date as the file's own date", () => {
		const rendered = formatBackupFileDate("2026-10-04T12:00:00Z");
		expect(rendered).toContain("Backup file from ");
		expect(rendered).toContain("2026");
	});

	test("admits an unparseable date instead of guessing", () => {
		expect(formatBackupFileDate("not-a-date")).toBe("Backup file date unknown.");
	});
});

describe("formatRestoreMetadataClause", () => {
	test("states metadata rides along when present", () => {
		expect(formatRestoreMetadataClause(true)).toContain("Includes Cratebug metadata");
	});

	test("states current metadata is kept when absent", () => {
		expect(formatRestoreMetadataClause(false)).toContain(
			"current tags and settings will be kept",
		);
	});
});
