import { describe, expect, it } from "bun:test";
import {
	accentPresets,
	defaultAccentPresetID,
	isSupportedAccentChoice,
	isValidHexColor,
	resolveAccent,
	selectedPresetID,
} from "./accentColor";

describe("resolveAccent", () => {
	it("resolves an empty choice to the default preset", () => {
		expect(resolveAccent("", "dark").hex).toBe("#a78bfa");
		expect(resolveAccent("", "light").hex).toBe("#7552d2");
	});

	it("resolves a preset id to its theme-specific hex", () => {
		expect(resolveAccent("coral", "dark").hex).toBe("#f47c67");
		expect(resolveAccent("coral", "light").hex).toBe("#bc4d3c");
	});

	it("passes a custom hex through unchanged for both themes", () => {
		expect(resolveAccent("#123456", "dark").hex).toBe("#123456");
		expect(resolveAccent("#123456", "light").hex).toBe("#123456");
	});

	it("picks a dark ink for every dark preset and a light ink for every light preset", () => {
		for (const preset of accentPresets) {
			expect(resolveAccent(preset.id, "dark").ink).toBe("#1b1408");
			expect(resolveAccent(preset.id, "light").ink).toBe("#fdfaf5");
		}
	});
});

describe("selectedPresetID", () => {
	it("treats an empty choice as the default preset", () => {
		expect(selectedPresetID("")).toBe(defaultAccentPresetID);
	});

	it("returns the matching preset id", () => {
		expect(selectedPresetID("azure")).toBe("azure");
	});

	it("returns null for a custom hex color", () => {
		expect(selectedPresetID("#123456")).toBeNull();
	});
});

describe("isSupportedAccentChoice", () => {
	it("accepts empty, known preset ids, and hex colors", () => {
		expect(isSupportedAccentChoice("")).toBe(true);
		expect(isSupportedAccentChoice("mint")).toBe(true);
		expect(isSupportedAccentChoice("#f0a54d")).toBe(true);
	});

	it("rejects unknown ids and case variants", () => {
		expect(isSupportedAccentChoice("orange")).toBe(false);
		expect(isSupportedAccentChoice("Iris")).toBe(false);
		expect(isSupportedAccentChoice("#fff")).toBe(false);
	});
});

describe("isValidHexColor", () => {
	it("accepts only 6-digit hex values", () => {
		expect(isValidHexColor("#f0a54d")).toBe(true);
		expect(isValidHexColor("f0a54d")).toBe(false);
		expect(isValidHexColor("#fff")).toBe(false);
	});
});
