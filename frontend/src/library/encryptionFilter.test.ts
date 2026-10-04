import { describe, expect, test } from "bun:test";
import { discovery, modtype } from "../../wailsjs/go/models";
import { type EncryptionFilter, matchesEncryptionFilter } from "./encryptionFilter";

function iostoreEntry(id: string): discovery.Entry {
	return new discovery.Entry({
		id,
		kind: "mod",
		bundleFormat: "iostore",
		primaryPath: `${id}.pak`,
		sidecars: { utoc: `${id}.utoc`, ucas: `${id}.ucas` },
		displayName: id,
		relativeFolder: "",
		state: "enabled",
		priority: { value: 0, kind: "none", raw: "", trailingNines: 0 },
	});
}

function classicEntry(id: string): discovery.Entry {
	return new discovery.Entry({
		id,
		kind: "mod",
		bundleFormat: "classic",
		primaryPath: `${id}.pak`,
		sidecars: {},
		displayName: id,
		relativeFolder: "",
		state: "enabled",
		priority: { value: 0, kind: "none", raw: "", trailingNines: 0 },
	});
}

function identity(encrypted: boolean): modtype.Identity {
	return new modtype.Identity({
		category: "Mesh",
		characterID: "",
		characterName: "",
		skinID: "",
		skinName: "",
		encrypted,
	});
}

describe("matchesEncryptionFilter", () => {
	test("all shows entries without an identity", () => {
		// Arrange
		const entry = iostoreEntry("a");

		// Act
		const visible = matchesEncryptionFilter(entry, "all", {});

		// Assert
		expect(visible).toBe(true);
	});

	test("encrypted shows only classified encrypted IoStore bundles", () => {
		// Arrange
		const identities = { a: identity(true), b: identity(false) };

		// Act / Assert
		expect(matchesEncryptionFilter(iostoreEntry("a"), "encrypted", identities)).toBe(true);
		expect(matchesEncryptionFilter(iostoreEntry("b"), "encrypted", identities)).toBe(false);
		expect(matchesEncryptionFilter(classicEntry("c"), "encrypted", identities)).toBe(false);
		expect(matchesEncryptionFilter(iostoreEntry("d"), "encrypted", identities)).toBe(false);
	});

	test("unencrypted hides classic bundles and entries still classifying", () => {
		// Arrange
		const identities = { a: identity(false), b: identity(true) };

		// Act / Assert
		expect(matchesEncryptionFilter(iostoreEntry("a"), "unencrypted", identities)).toBe(true);
		expect(matchesEncryptionFilter(iostoreEntry("b"), "unencrypted", identities)).toBe(false);
		expect(matchesEncryptionFilter(classicEntry("c"), "unencrypted", identities)).toBe(false);
		expect(matchesEncryptionFilter(iostoreEntry("d"), "unencrypted", identities)).toBe(false);
	});

	test.each(["encrypted", "unencrypted"] as EncryptionFilter[])(
		"%s hides an incomplete IoStore bundle",
		(filter) => {
			// Arrange
			const entry = new discovery.Entry({
				id: "partial",
				kind: "mod",
				bundleFormat: "iostore",
				primaryPath: "partial.pak",
				sidecars: { utoc: "partial.utoc" },
				displayName: "partial",
				relativeFolder: "",
				state: "enabled",
				priority: { value: 0, kind: "none", raw: "", trailingNines: 0 },
			});

			// Act
			const visible = matchesEncryptionFilter(entry, filter, {
				partial: identity(false),
			});

			// Assert
			expect(visible).toBe(false);
		},
	);
});
