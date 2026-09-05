import type { discovery } from "../../wailsjs/go/models";

// Mirrors internal/mutation's Windows filename component limit (see maximumFileNameUTF16CodeUnits).
const maximumFileNameUTF16CodeUnits = 255;
// Mirrors discovery.MinimumTrailingNines, the shortest trailing-nine priority form.
const minimumTrailingNines = 7;

// The trailing-nine priority filename grows with the priority value itself
// (more nines), so the true ceiling depends on the mod's own name length and
// its longest present file suffix, not a single constant shared by every mod.
export function maximumPriorityFor(entry: discovery.Entry): number {
	const currentStemLength = entry.priority.raw.length;
	const suffixLengths = [entry.primaryPath, entry.sidecars.utoc, entry.sidecars.ucas]
		.filter((path): path is string => path !== undefined)
		.map((path) => basename(path).length - currentStemLength);
	const longestSuffixLength = Math.max(0, ...suffixLengths);

	// Stem length for a positive priority is name + "_" + nines + "_P".
	const fixedStemOverhead = entry.displayName.length + 1 + (minimumTrailingNines - 1) + 2;
	const ceiling = maximumFileNameUTF16CodeUnits - longestSuffixLength - fixedStemOverhead;
	return Math.max(0, Math.min(maximumFileNameUTF16CodeUnits, ceiling));
}

function basename(path: string): string {
	return path.split(/[/\\]/).pop() ?? path;
}
