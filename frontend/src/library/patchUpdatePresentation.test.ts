import { describe, expect, test } from "bun:test";

import { mutation } from "../../wailsjs/go/models";
import {
	countPatchUpdateMods,
	formatPatchUpdateFromBuilds,
	formatPatchUpdateProgress,
	groupPatchUpdatePlan,
	groupPatchUpdateResult,
	patchUpdateModLine,
	patchUpdateScopeLabel,
	selectablePatchUpdateMods,
	summarizePatchUpdatePlan,
	summarizePatchUpdateResult,
} from "./patchUpdatePresentation";

function mod(source: Partial<mutation.PatchUpdateMod>): mutation.PatchUpdateMod {
	return new mutation.PatchUpdateMod({
		entryID: "mod",
		displayName: "Mod",
		relativeFolder: "",
		enabled: true,
		status: "current",
		updated: 0,
		...source,
	});
}

function plan(mods: mutation.PatchUpdateMod[]): mutation.PatchUpdatePlan {
	return new mutation.PatchUpdatePlan({
		gameBuild: 4008108,
		gamePaks: "C:/Rivals/Paks",
		mods,
	});
}

function result(mods: mutation.PatchUpdateMod[], cancelled = false): mutation.PatchUpdateResult {
	return new mutation.PatchUpdateResult({
		gameBuild: 4008108,
		originalsDir: "C:/originals",
		mods,
		cancelled,
	});
}

describe("patch update plan", () => {
	const mods = [
		mod({ entryID: "a", displayName: "Astral", status: "updated", updated: 8 }),
		mod({
			entryID: "b",
			displayName: "Broken",
			status: "partial",
			updated: 2,
			blocked: ["x: no"],
		}),
		mod({ entryID: "c", displayName: "Hela", status: "blocked", blocked: ["y: no"] }),
		mod({ entryID: "d", displayName: "Luna", status: "ambiguous", ambiguous: ["z"] }),
		mod({ entryID: "e", displayName: "Mantis" }),
	];

	test("selects fully and partly updatable mods", () => {
		expect(selectablePatchUpdateMods(plan(mods)).map((entry) => entry.displayName)).toEqual([
			"Astral",
			"Broken",
		]);
	});

	test("counts every outcome", () => {
		expect(countPatchUpdateMods(mods)).toEqual({
			updated: 1,
			partial: 1,
			blocked: 1,
			ambiguous: 1,
			failed: 0,
			current: 1,
			updatable: 2,
		});
	});

	test("names the scope of a check", () => {
		expect(patchUpdateScopeLabel(0)).toBe("entire library");
		expect(patchUpdateScopeLabel(1)).toBe("1 mod");
		expect(patchUpdateScopeLabel(12)).toBe("12 selected mods");
	});

	test("summarizes the check before selection", () => {
		expect(summarizePatchUpdatePlan(plan(mods))).toBe(
			"Checked 5 complete IoStore mods for build 4008108: 2 mods can be updated (1 only partly), 1 mod needs its author, 1 mod is ambiguous, 1 already current.",
		);
	});

	test("groups updatable mods together and keeps current mods out of the cards", () => {
		const groups = groupPatchUpdatePlan(plan(mods));
		expect(groups.map((group) => [group.key, group.mods.length])).toEqual([
			["updatable", 2],
			["blocked", 1],
			["ambiguous", 1],
			["current", 1],
		]);
		expect(groups[0]?.selectable).toBe(true);
		expect(groups[1]?.selectable).toBe(false);
	});
});

describe("patch update result", () => {
	test("summarizes what happened, including partial updates", () => {
		expect(
			summarizePatchUpdateResult(
				result([
					mod({ status: "updated", updated: 8 }),
					mod({ status: "partial", updated: 1, blocked: ["a: no"] }),
					mod({ status: "blocked", blocked: ["b: no"] }),
					mod({ status: "failed", detail: "worker stopped" }),
				]),
			),
		).toBe("2 mods updated (1 only partly), 1 mod needs its author, 1 mod failed.");
	});

	test("says when the user cancelled", () => {
		expect(
			summarizePatchUpdateResult(result([mod({ status: "updated", updated: 1 })], true)),
		).toBe("Cancelled. 1 mod updated.");
	});

	test("groups partial, blocked, and failed results separately", () => {
		const groups = groupPatchUpdateResult(
			result([
				mod({ status: "updated", updated: 1 }),
				mod({ status: "partial", updated: 1, blocked: ["a: no"] }),
				mod({ status: "blocked" }),
				mod({ status: "failed" }),
			]),
		);
		expect(groups.map((group) => group.key)).toEqual([
			"updated",
			"partial",
			"blocked",
			"failed",
		]);
	});
});

describe("patch update mod lines", () => {
	test("uses plan language before applying", () => {
		expect(
			patchUpdateModLine(
				mod({ status: "partial", updated: 2, blocked: ["a: no", "b: no"] }),
				"plan",
			),
		).toBe("2 files can be carried; 2 files still need their author.");
	});

	test("uses result language after applying", () => {
		expect(patchUpdateModLine(mod({ status: "updated", updated: 3 }), "result")).toBe(
			"3 files carried to the current layout.",
		);
	});
});

test("names the earlier layouts", () => {
	expect(formatPatchUpdateFromBuilds([3984769])).toBe("Older layout: build 3984769");
	expect(formatPatchUpdateFromBuilds([3984769, 3962284])).toBe("Older layouts: 3984769, 3962284");
	expect(formatPatchUpdateFromBuilds(undefined)).toBe("");
});

test("formatPatchUpdateProgress names the phase and mod", () => {
	expect(
		formatPatchUpdateProgress({
			current: 3,
			total: 170,
			entryID: "mod",
			displayName: "Luna",
			phase: "checking",
		}),
	).toBe("Checking 3 of 170 — Luna…");
	expect(
		formatPatchUpdateProgress({
			current: 2,
			total: 12,
			entryID: "mod",
			displayName: "Hela",
			phase: "updating",
		}),
	).toBe("Updating 2 of 12 — Hela…");
});
