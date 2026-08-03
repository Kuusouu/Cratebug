export type LibraryState = "initial" | "loading" | "populated" | "empty" | "error";

export const viewModes = ["compact", "large", "list"] as const;

export type ViewMode = (typeof viewModes)[number];

export const viewModeLabels: Record<ViewMode, string> = {
	compact: "Compact",
	large: "Large",
	list: "List",
};
