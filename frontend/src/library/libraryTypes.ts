export type LibraryState = "initial" | "loading" | "populated" | "empty" | "error";

export const viewModes = ["compact", "large", "list"] as const;

export type ViewMode = (typeof viewModes)[number];

export const viewModeLabels: Record<ViewMode, string> = {
	compact: "Compact",
	large: "Large",
	list: "List",
};

export const themes = ["system", "light", "dark"] as const;

export type Theme = (typeof themes)[number];

export const themeLabels: Record<Theme, string> = {
	system: "System",
	light: "Light",
	dark: "Dark",
};
