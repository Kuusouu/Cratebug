import type { mutation } from "../../wailsjs/go/models";

export type PatchUpdateProgressView = {
	current: number;
	total: number;
	entryID: string;
	displayName: string;
	phase: string;
};

export type PatchUpdatePlanGroup = {
	key: "updatable" | "blocked" | "ambiguous" | "failed" | "current";
	title: string;
	description: string;
	selectable: boolean;
	mods: mutation.PatchUpdateMod[];
};

export type PatchUpdateResultGroup = {
	key: "updated" | "partial" | "blocked" | "ambiguous" | "failed" | "current";
	title: string;
	description: string;
	mods: mutation.PatchUpdateMod[];
};

export type PatchUpdateCounts = {
	updated: number;
	partial: number;
	blocked: number;
	ambiguous: number;
	failed: number;
	current: number;
	updatable: number;
};

// The mods a check says can be carried to the current layout. Partly
// updatable mods are included: their remaining files are reported separately.
export function selectablePatchUpdateMods(
	plan: mutation.PatchUpdatePlan,
): mutation.PatchUpdateMod[] {
	return plan.mods.filter((mod) => mod.status === "updated" || mod.status === "partial");
}

export function countPatchUpdateMods(mods: mutation.PatchUpdateMod[]): PatchUpdateCounts {
	const counts: PatchUpdateCounts = {
		updated: 0,
		partial: 0,
		blocked: 0,
		ambiguous: 0,
		failed: 0,
		current: 0,
		updatable: 0,
	};
	for (const mod of mods) {
		switch (mod.status) {
			case "updated":
				counts.updated++;
				break;
			case "partial":
				counts.partial++;
				break;
			case "blocked":
				counts.blocked++;
				break;
			case "ambiguous":
				counts.ambiguous++;
				break;
			case "failed":
				counts.failed++;
				break;
			case "current":
				counts.current++;
				break;
		}
	}
	counts.updatable = counts.updated + counts.partial;
	return counts;
}

// Names the scope of a check for the dialog header.
export function patchUpdateScopeLabel(targetCount: number): string {
	if (targetCount === 0) return "entire library";
	if (targetCount === 1) return "1 mod";
	return `${targetCount} selected mods`;
}

// One line for the check result, before the user selects anything.
export function summarizePatchUpdatePlan(plan: mutation.PatchUpdatePlan): string {
	const counts = countPatchUpdateMods(plan.mods);
	const parts = [`${plural(counts.updatable, "mod")} can be updated`];
	if (counts.partial > 0) {
		parts[0] += ` (${counts.partial} only partly)`;
	}
	if (counts.blocked > 0) {
		parts.push(`${plural(counts.blocked, "mod")} ${needsAuthorVerb(counts.blocked)}`);
	}
	if (counts.ambiguous > 0) {
		parts.push(
			`${plural(counts.ambiguous, "mod")} ${counts.ambiguous === 1 ? "is" : "are"} ambiguous`,
		);
	}
	if (counts.failed > 0) {
		parts.push(`${plural(counts.failed, "mod")} could not be checked`);
	}
	parts.push(`${counts.current} already current`);
	return `Checked ${plural(plan.mods.length, "complete IoStore mod")} for build ${plan.gameBuild}: ${parts.join(", ")}.`;
}

// One line for what an apply actually did.
export function summarizePatchUpdateResult(result: mutation.PatchUpdateResult): string {
	const counts = countPatchUpdateMods(result.mods);
	const parts = [`${plural(counts.updatable, "mod")} updated`];
	if (counts.partial > 0) {
		parts[0] += ` (${counts.partial} only partly)`;
	}
	if (counts.blocked > 0) {
		parts.push(`${plural(counts.blocked, "mod")} ${needsAuthorVerb(counts.blocked)}`);
	}
	if (counts.failed > 0) {
		parts.push(`${plural(counts.failed, "mod")} failed`);
	}
	if (counts.ambiguous > 0) {
		parts.push(`${plural(counts.ambiguous, "mod")} left ambiguous`);
	}
	const prefix = result.cancelled ? "Cancelled. " : "";
	return `${prefix}${capitalize(parts.join(", "))}.`;
}

// Groups a check's mods for the selection list. "current" is kept so the UI
// can report the count without rendering every current mod as a card.
export function groupPatchUpdatePlan(plan: mutation.PatchUpdatePlan): PatchUpdatePlanGroup[] {
	const groups: PatchUpdatePlanGroup[] = [
		{
			key: "updatable",
			title: "Can be updated",
			description:
				"These mods use an older layout. Select the ones to carry to the current build.",
			selectable: true,
			mods: [],
		},
		{
			key: "blocked",
			title: "Need their author",
			description: "Cratebug cannot make these changes safely, so they stay as they are.",
			selectable: false,
			mods: [],
		},
		{
			key: "ambiguous",
			title: "Ambiguous",
			description:
				"Cratebug cannot tell which layout these use, so it leaves them as they are.",
			selectable: false,
			mods: [],
		},
		{
			key: "failed",
			title: "Could not be checked",
			description: "Cratebug could not read these mods.",
			selectable: false,
			mods: [],
		},
		{
			key: "current",
			title: "Already current",
			description: "These mods already use the current layout.",
			selectable: false,
			mods: [],
		},
	];
	for (const mod of plan.mods) {
		const key = mod.status === "updated" || mod.status === "partial" ? "updatable" : mod.status;
		const group = groups.find((candidate) => candidate.key === key);
		if (group) group.mods.push(mod);
	}
	return groups.filter((group) => group.mods.length > 0);
}

// Groups the mods an apply processed by what happened to them.
export function groupPatchUpdateResult(
	result: mutation.PatchUpdateResult,
): PatchUpdateResultGroup[] {
	const groups: PatchUpdateResultGroup[] = [
		{
			key: "updated",
			title: "Updated",
			description: "Every file that needed a new layout was carried across.",
			mods: [],
		},
		{
			key: "partial",
			title: "Partly updated",
			description: "Some files were carried; the rest still need their author.",
			mods: [],
		},
		{
			key: "blocked",
			title: "Still need their author",
			description: "Nothing was changed in these mods.",
			mods: [],
		},
		{
			key: "ambiguous",
			title: "Left as they were",
			description: "Cratebug could not tell which layout these use.",
			mods: [],
		},
		{
			key: "failed",
			title: "Failed",
			description: "Cratebug could not update these mods.",
			mods: [],
		},
		{
			key: "current",
			title: "No change needed",
			description: "These mods already used the current layout.",
			mods: [],
		},
	];
	for (const mod of result.mods) {
		const group = groups.find((candidate) => candidate.key === mod.status);
		if (group) group.mods.push(mod);
	}
	return groups.filter((group) => group.mods.length > 0);
}

// The sentence under a mod's name. Plan wording says what could happen;
// result wording says what did.
export function patchUpdateModLine(mod: mutation.PatchUpdateMod, phase: "plan" | "result"): string {
	const blocked = mod.blocked?.length ?? 0;
	const ambiguous = mod.ambiguous?.length ?? 0;
	if (phase === "plan") {
		switch (mod.status) {
			case "updated":
				return `${plural(mod.updated, "file")} can be carried to the current layout.`;
			case "partial":
				return `${plural(mod.updated, "file")} can be carried; ${needAuthorClause(blocked)}.`;
			case "blocked":
				return "Nothing can be updated safely.";
			case "ambiguous":
				return "Cratebug cannot tell which layout this mod uses.";
			case "failed":
				return mod.detail ?? "This mod could not be checked.";
			case "current":
				return "Already uses the current layout.";
		}
	}
	switch (mod.status) {
		case "updated":
			return `${plural(mod.updated, "file")} carried to the current layout.`;
		case "partial":
			return `${plural(mod.updated, "file")} carried; ${needAuthorClause(blocked)}.`;
		case "blocked":
			return "No files were changed.";
		case "ambiguous":
			return "Left as it was.";
		case "failed":
			return mod.detail ?? "This mod could not be updated.";
		case "current":
			return "No change was needed.";
	}
	if (ambiguous > 0) return `${plural(ambiguous, "file")} could not be identified.`;
	return "";
}

// Names the earlier layout a mod's assets came from, when it is known.
export function formatPatchUpdateFromBuilds(fromBuilds: number[] | undefined): string {
	if (!fromBuilds || fromBuilds.length === 0) return "";
	if (fromBuilds.length === 1) return `Older layout: build ${fromBuilds[0]}`;
	return `Older layouts: ${fromBuilds.join(", ")}`;
}

export function formatPatchUpdateProgress(progress: PatchUpdateProgressView): string {
	const verb = progress.phase === "updating" ? "Updating" : "Checking";
	return `${verb} ${progress.current} of ${progress.total} — ${progress.displayName}…`;
}

function needsAuthorVerb(count: number): string {
	return count === 1 ? "needs its author" : "need their author";
}

function needAuthorClause(count: number): string {
	if (count === 0) return "nothing else needs attention";
	return `${plural(count, "file")} ${count === 1 ? "still needs its author" : "still need their author"}`;
}

function plural(count: number, noun: string): string {
	return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

function capitalize(value: string): string {
	return value.length === 0 ? value : value.charAt(0).toUpperCase() + value.slice(1);
}
