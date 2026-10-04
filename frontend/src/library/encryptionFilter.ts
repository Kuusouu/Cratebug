import type { discovery, modtype } from "../../wailsjs/go/models";
import { canEncryptMod } from "./encryptionAction";

/** Which encryption states the catalog shows. */
export type EncryptionFilter = "all" | "encrypted" | "unencrypted";

export const encryptionFilters: readonly EncryptionFilter[] = ["all", "encrypted", "unencrypted"];

export const encryptionFilterLabels: Record<EncryptionFilter, string> = {
	all: "All",
	encrypted: "Encrypted",
	unencrypted: "Unencrypted",
};

/**
 * True when the entry stays visible under the filter. "all" shows everything,
 * including entries still waiting on classification and bundles that cannot
 * be encrypted. The narrowed states show only classified complete IoStore
 * bundles, so classic and incomplete bundles never read as "unencrypted" and
 * unclassified entries stay hidden until their state is known.
 */
export function matchesEncryptionFilter(
	entry: discovery.Entry,
	filter: EncryptionFilter,
	identitiesByEntryID: Record<string, modtype.Identity>,
): boolean {
	if (filter === "all") {
		return true;
	}
	if (!canEncryptMod(entry)) {
		return false;
	}
	const identity = identitiesByEntryID[entry.id];
	if (!identity) {
		return false;
	}
	return filter === "encrypted" ? identity.encrypted : !identity.encrypted;
}
