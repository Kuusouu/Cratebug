import type { ResolvedTheme } from "./libraryTypes";

export type AccentPreset = {
	id: string;
	name: string;
	dark: string;
	light: string;
};

// Cratebug's Midnight Iris presets. Each carries a separate light and dark
// hex so a chosen preset keeps its identity across themes: deeper tones on
// the light canvas, brighter ones on the dark one.
export const accentPresets: AccentPreset[] = [
	{ id: "iris", name: "Iris", dark: "#a78bfa", light: "#7552d2" },
	{ id: "coral", name: "Coral", dark: "#f47c67", light: "#bc4d3c" },
	{ id: "amber", name: "Amber", dark: "#e9b568", light: "#946100" },
	{ id: "mint", name: "Mint", dark: "#36cdb4", light: "#0c816f" },
	{ id: "rose", name: "Rose", dark: "#ec7099", light: "#b72f68" },
	{ id: "azure", name: "Azure", dark: "#65a8f8", light: "#2566b8" },
];

// The preset a fresh profile starts with. Documents written before presets
// existed carry an empty accent value, which resolves to this too.
export const defaultAccentPresetID = "iris";

const hexColorPattern = /^#[0-9a-fA-F]{6}$/;

export function isValidHexColor(value: string): boolean {
	return hexColorPattern.test(value);
}

// Picks readable text for an arbitrary accent background. The shell's static
// --accent-ink values match the default preset; any other accent (another
// preset or a custom hex) needs its own contrast decision instead of
// inheriting those.
export function contrastingInk(hex: string): string {
	const r = Number.parseInt(hex.slice(1, 3), 16);
	const g = Number.parseInt(hex.slice(3, 5), 16);
	const b = Number.parseInt(hex.slice(5, 7), 16);
	const perceivedBrightness = r * 0.299 + g * 0.587 + b * 0.114;
	return perceivedBrightness > 150 ? "#1b1408" : "#fdfaf5";
}

function presetByID(id: string): AccentPreset | undefined {
	return accentPresets.find((preset) => preset.id === id);
}

// Whether a stored accent value is one this build understands: empty for the
// default preset, a known preset id, or a custom hex color.
export function isSupportedAccentChoice(accent: string): boolean {
	return accent === "" || isValidHexColor(accent) || presetByID(accent) !== undefined;
}

// The preset id the swatch picker marks as selected, or null when the stored
// value is a custom hex color. An empty value means the default preset.
export function selectedPresetID(accent: string): string | null {
	if (accent === "") return defaultAccentPresetID;
	return presetByID(accent)?.id ?? null;
}

// The concrete accent to paint for a stored choice: a preset resolves to its
// dark or light hex, a custom hex passes through unchanged.
export function resolveAccent(accent: string, theme: ResolvedTheme): { hex: string; ink: string } {
	const preset = presetByID(accent === "" ? defaultAccentPresetID : accent);
	const hex = preset === undefined ? accent : theme === "dark" ? preset.dark : preset.light;
	return { hex, ink: contrastingInk(hex) };
}
