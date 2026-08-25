import { describe, expect, it } from "bun:test";
import { discovery, modtype } from "../../wailsjs/go/models";
import {
	canChangeModState,
	canDeleteMod,
	canOrganizeMod,
	canTagMod,
	categorySlug,
	entryCategoryLabel,
	entryCharacterLabel,
	entryStateLabel,
} from "./entryPresentation";

describe("entryCategoryLabel", () => {
	it("returns empty string for undefined or null identity", () => {
		expect(entryCategoryLabel(undefined)).toBe("");
		expect(entryCategoryLabel(null)).toBe("");
	});

	it("returns empty string when category is missing or empty", () => {
		const identity = new modtype.Identity({ category: "" });
		expect(entryCategoryLabel(identity)).toBe("");
	});

	it("returns Unknown for Unknown category", () => {
		const identity = new modtype.Identity({ category: "Unknown" });
		expect(entryCategoryLabel(identity)).toBe("Unknown");
	});

	it("returns category name for recognized categories", () => {
		expect(entryCategoryLabel(new modtype.Identity({ category: "Mesh" }))).toBe("Mesh");
		expect(entryCategoryLabel(new modtype.Identity({ category: "Audio" }))).toBe("Audio");
		expect(entryCategoryLabel(new modtype.Identity({ category: "UI" }))).toBe("UI");
		expect(entryCategoryLabel(new modtype.Identity({ category: "VFX" }))).toBe("VFX");
	});
});

describe("categorySlug", () => {
	it("returns empty string for falsy category", () => {
		expect(categorySlug(undefined)).toBe("");
		expect(categorySlug(null)).toBe("");
		expect(categorySlug("")).toBe("");
	});

	it("converts single words to lowercase", () => {
		expect(categorySlug("Mesh")).toBe("mesh");
		expect(categorySlug("VFX")).toBe("vfx");
	});

	it("replaces spaces with dashes", () => {
		expect(categorySlug("Static Mesh")).toBe("static-mesh");
	});
});

describe("entryCharacterLabel", () => {
	it("returns empty string when characterName is missing or empty", () => {
		expect(entryCharacterLabel(undefined)).toBe("");
		expect(entryCharacterLabel(new modtype.Identity({ category: "Mesh" }))).toBe("");
	});

	it("returns character name when skinName is absent", () => {
		const identity = new modtype.Identity({
			category: "Mesh",
			characterName: "Blade",
		});
		expect(entryCharacterLabel(identity)).toBe("Blade");
	});

	it("returns character and skin name when both are present", () => {
		const identity = new modtype.Identity({
			category: "Mesh",
			characterName: "Blade",
			skinName: "Daywalker",
		});
		expect(entryCharacterLabel(identity)).toBe("Blade (Daywalker)");
	});
});

describe("entryStateLabel", () => {
	it("returns Orphaned sidecar for orphaned entries", () => {
		const entry = new discovery.Entry({
			kind: "orphaned_sidecar",
			state: "enabled",
		});
		expect(entryStateLabel(entry)).toBe("Orphaned sidecar");
	});

	it("returns Enabled for enabled mods", () => {
		const entry = new discovery.Entry({
			kind: "mod",
			state: "enabled",
		});
		expect(entryStateLabel(entry)).toBe("Enabled");
	});

	it("returns Disabled for disabled mods", () => {
		const entry = new discovery.Entry({
			kind: "mod",
			state: "disabled",
		});
		expect(entryStateLabel(entry)).toBe("Disabled");
	});
});

describe("capability predicates", () => {
	it("canChangeModState requires mod kind, primaryPath, and no ambiguous primary", () => {
		const validMod = new discovery.Entry({
			kind: "mod",
			primaryPath: "mod.pak",
		});
		expect(canChangeModState(validMod)).toBe(true);

		const orphaned = new discovery.Entry({
			kind: "orphaned_sidecar",
			primaryPath: undefined,
		});
		expect(canChangeModState(orphaned)).toBe(false);

		const ambiguous = new discovery.Entry({
			kind: "mod",
			primaryPath: "mod.pak",
			issues: [new discovery.Issue({ code: "ambiguous-primary", message: "Ambiguous" })],
		});
		expect(canChangeModState(ambiguous)).toBe(false);
	});

	it("canOrganizeMod rejects missing sidecars", () => {
		const complete = new discovery.Entry({
			kind: "mod",
			primaryPath: "mod.pak",
		});
		expect(canOrganizeMod(complete)).toBe(true);

		const missingUtoc = new discovery.Entry({
			kind: "mod",
			primaryPath: "mod.pak",
			issues: [new discovery.Issue({ code: "missing-utoc", message: "Missing utoc" })],
		});
		expect(canOrganizeMod(missingUtoc)).toBe(false);
	});

	it("canDeleteMod allows deleting primary-backed mods even with missing sidecars", () => {
		const missingUtoc = new discovery.Entry({
			kind: "mod",
			primaryPath: "mod.pak",
			issues: [new discovery.Issue({ code: "missing-utoc", message: "Missing utoc" })],
		});
		expect(canDeleteMod(missingUtoc)).toBe(true);

		const orphaned = new discovery.Entry({
			kind: "orphaned_sidecar",
			primaryPath: undefined,
		});
		expect(canDeleteMod(orphaned)).toBe(false);
	});

	it("canTagMod allows any mod kind entry", () => {
		expect(canTagMod(new discovery.Entry({ kind: "mod" }))).toBe(true);
		expect(canTagMod(new discovery.Entry({ kind: "orphaned_sidecar" }))).toBe(false);
	});
});
