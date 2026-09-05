import { describe, expect, test } from "bun:test";
import { discovery, modtype } from "../../wailsjs/go/models";
import { canEncryptMod, encryptionMenuState } from "./encryptionAction";

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

describe("canEncryptMod", () => {
	test("accepts a complete IoStore bundle", () => {
		expect(canEncryptMod(iostoreEntry("a"))).toBe(true);
	});

	test("rejects a classic PAK", () => {
		expect(canEncryptMod(classicEntry("a"))).toBe(false);
	});
});

describe("encryptionMenuState", () => {
	test("encrypts when every checked IoStore is clear", () => {
		const state = encryptionMenuState([iostoreEntry("a")], { a: identity(false) });
		expect(state.kind).toBe("encrypt");
		expect(state.encrypt).toBe(true);
	});

	test("decrypts when every checked IoStore is encrypted", () => {
		const state = encryptionMenuState([iostoreEntry("a")], { a: identity(true) });
		expect(state.kind).toBe("decrypt");
		expect(state.encrypt).toBe(false);
	});

	test("disables mixed encryption", () => {
		const state = encryptionMenuState([iostoreEntry("a"), iostoreEntry("b")], {
			a: identity(true),
			b: identity(false),
		});
		expect(state.kind).toBe("mixed");
	});

	test("disables classic members", () => {
		const state = encryptionMenuState([iostoreEntry("a"), classicEntry("b")], {
			a: identity(false),
		});
		expect(state.kind).toBe("ineligible");
	});
});
