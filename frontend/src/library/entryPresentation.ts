import type { discovery } from "../../wailsjs/go/models";

// Keeps state wording consistent wherever a discovered entry is presented.
export function entryStateLabel(entry: discovery.Entry): string {
	if (entry.kind === "orphaned_sidecar") return "Orphaned sidecar";

	return entry.state === "disabled" ? "Disabled" : "Enabled";
}
