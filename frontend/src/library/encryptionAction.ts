import type { discovery, modtype } from "../../wailsjs/go/models";
import { hasAmbiguousPrimary, hasMissingSidecar } from "./entryPresentation";

/** Why Encrypt/Decrypt is enabled or disabled for the checked set. */
export type EncryptionMenuKind =
	| "empty"
	| "classifying"
	| "ineligible"
	| "mixed"
	| "encrypt"
	| "decrypt";

/** Menu label and disable reason for the Encrypt/Decrypt action. */
export type EncryptionMenuState = {
	kind: EncryptionMenuKind;
	reason: string;
	encrypt: boolean;
};

const mixedReason = "Mixed encryption state. Select only encrypted or only unencrypted mods.";
const ineligibleReason = "Encrypt only works on complete IoStore mods.";
const classifyingReason = "Still classifying encryption state.";
const emptyReason = "Select mods first.";

/** True when entry is a complete IoStore bundle that encrypt/decrypt can rebuild. */
export function canEncryptMod(entry: discovery.Entry): boolean {
	return (
		entry.kind === "mod" &&
		entry.bundleFormat === "iostore" &&
		Boolean(entry.sidecars?.utoc) &&
		Boolean(entry.sidecars?.ucas) &&
		!hasAmbiguousPrimary(entry) &&
		!hasMissingSidecar(entry)
	);
}

/** Derives Encrypt vs Decrypt vs a disabled reason from the checked identities. */
export function encryptionMenuState(
	entries: discovery.Entry[],
	identitiesByEntryID: Record<string, modtype.Identity>,
): EncryptionMenuState {
	if (entries.length === 0) {
		return { kind: "empty", reason: emptyReason, encrypt: true };
	}
	if (entries.some((entry) => !canEncryptMod(entry))) {
		return { kind: "ineligible", reason: ineligibleReason, encrypt: true };
	}
	if (entries.some((entry) => !identitiesByEntryID[entry.id])) {
		return { kind: "classifying", reason: classifyingReason, encrypt: true };
	}

	const encryptedCount = entries.filter(
		(entry) => identitiesByEntryID[entry.id]?.encrypted,
	).length;
	if (encryptedCount !== 0 && encryptedCount !== entries.length) {
		return { kind: "mixed", reason: mixedReason, encrypt: true };
	}
	if (encryptedCount === entries.length) {
		return { kind: "decrypt", reason: "", encrypt: false };
	}
	return { kind: "encrypt", reason: "", encrypt: true };
}
