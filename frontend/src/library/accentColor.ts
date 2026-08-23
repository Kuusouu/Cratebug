export type AccentPreset = {
	name: string;
	hex: string;
};

// Cratebug's own choices, not BentoMod's swatch values: a warm ember close to
// the current default, a crate/lootbox-toned cream, then a spread of hues
// distinct enough to tell apart at a glance.
export const accentPresets: AccentPreset[] = [
	{ name: "Ember", hex: "#e8703a" },
	{ name: "Crate", hex: "#d9a066" },
	{ name: "Violet", hex: "#8b5cf6" },
	{ name: "Teal", hex: "#2dd4bf" },
	{ name: "Crimson", hex: "#e63950" },
];

const hexColorPattern = /^#[0-9a-fA-F]{6}$/;

export function isValidHexColor(value: string): boolean {
	return hexColorPattern.test(value);
}

// Picks readable text for an arbitrary accent background. The app's own
// --accent-ink is tuned for each theme's specific default orange, so a custom
// accent needs its own contrast decision rather than inheriting that value.
export function contrastingInk(hex: string): string {
	const r = Number.parseInt(hex.slice(1, 3), 16);
	const g = Number.parseInt(hex.slice(3, 5), 16);
	const b = Number.parseInt(hex.slice(5, 7), 16);
	const perceivedBrightness = r * 0.299 + g * 0.587 + b * 0.114;
	return perceivedBrightness > 150 ? "#1b1408" : "#fdfaf5";
}
