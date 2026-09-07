import { formatBytes, formatWailsError } from "./installPresentation";

export const nexusGameDomain = "marvelrivals";
export const nexusModsSiteOrigin = "https://www.nexusmods.com";

const maxNexusPageURLLength = 2048;
const nexusModsHosts = new Set(["nexusmods.com", "www.nexusmods.com"]);

export type NexusAccountView = {
	configured: boolean;
	verified: boolean;
	isPremium: boolean;
};

export type NexusFileView = {
	fileId: number;
	name: string;
	fileName: string;
	version: string;
	sizeKb: number;
	categoryName: string;
	isPrimary: boolean;
};

export type NexusLinkView = {
	present: boolean;
	modId?: number;
	fileId?: number;
	modName?: string;
	author?: string;
	fileName?: string;
	version?: string;
	sizeKb?: number;
	needsWebsite: boolean;
	files?: NexusFileView[];
};

export type InstallProgressView = {
	phase?: string;
	message?: string;
	percent?: number;
};

export type NexusPageParse = { ok: true; url: string } | { ok: false; error: string };

export type NexusInstallStep =
	| "connect"
	| "paste"
	| "pick-file"
	| "no-files"
	| "needs-website"
	| "ready";

export function nexusModPageURL(modId: number, fileId?: number): string {
	const base = `${nexusModsSiteOrigin}/${nexusGameDomain}/mods/${modId}?tab=files`;
	if (fileId && fileId > 0) {
		return `${base}&file_id=${fileId}`;
	}
	return base;
}

export function parseNexusModPageInput(raw: string): NexusPageParse {
	const trimmed = raw.trim();
	if (trimmed === "") {
		return { ok: false, error: "Enter a Nexus Mods page URL." };
	}
	if (trimmed.length > maxNexusPageURLLength) {
		return { ok: false, error: "That URL is too long." };
	}
	if (trimmed.includes('"')) {
		return { ok: false, error: "That URL contains an unsafe character." };
	}

	let parsed: URL;
	try {
		parsed = new URL(trimmed);
	} catch {
		return { ok: false, error: "Enter a valid Nexus Mods page URL." };
	}

	if (parsed.protocol === "nxm:") {
		return {
			ok: false,
			error: "Paste a Nexus Mods page URL, or click Mod Manager Download in your browser.",
		};
	}
	if (parsed.protocol !== "https:") {
		return { ok: false, error: "The URL must start with https://." };
	}
	if (parsed.username !== "" || parsed.password !== "") {
		return { ok: false, error: "Enter a valid Nexus Mods page URL." };
	}

	const host = parsed.hostname.toLowerCase();
	if (!nexusModsHosts.has(host)) {
		return { ok: false, error: "Enter a Nexus Mods page URL." };
	}

	const path = parsed.pathname.toLowerCase();
	if (!path.includes(`/${nexusGameDomain}/`) || !path.includes("/mods/")) {
		return { ok: false, error: "Cratebug only installs Marvel Rivals mods from Nexus Mods." };
	}

	return { ok: true, url: trimmed };
}

export function nexusInstallStep(
	account: NexusAccountView | null,
	link: NexusLinkView | null,
): NexusInstallStep {
	if (!account?.configured || !account.verified) return "connect";
	if (!link?.present) return "paste";
	if (!link.fileId && (link.files?.length ?? 0) > 0) return "pick-file";
	if (!link.fileId) return "no-files";
	if (link.needsWebsite) return "needs-website";
	return "ready";
}

export function defaultNexusFileId(files: readonly NexusFileView[]): number | null {
	const primary = files.find((file) => file.isPrimary);
	return primary?.fileId ?? files[0]?.fileId ?? null;
}

export function groupNexusFiles(files: readonly NexusFileView[]): {
	main: NexusFileView[];
	optional: NexusFileView[];
} {
	const main: NexusFileView[] = [];
	const optional: NexusFileView[] = [];
	for (const file of files) {
		if (file.categoryName.toLowerCase() === "optional") {
			optional.push(file);
		} else {
			main.push(file);
		}
	}
	return { main, optional };
}

export function formatNexusFileSize(sizeKb: number): string {
	if (sizeKb <= 0) return "Unknown size";
	return formatBytes(sizeKb * 1024);
}

export function formatNexusFileLabel(file: NexusFileView): string {
	const size = formatNexusFileSize(file.sizeKb);
	const version = file.version.trim();
	if (version !== "") {
		return `${file.name} · ${version} · ${size}`;
	}
	return `${file.name} · ${size}`;
}

export function formatNexusLinkTitle(link: NexusLinkView): string {
	if (link.modName && link.modName.trim() !== "") return link.modName;
	if (link.modId && link.modId > 0) return `Mod ${link.modId}`;
	return "Nexus Mods download";
}

export function formatInstallProgress(progress: InstallProgressView | null): string {
	if (!progress) return "Downloading from Nexus Mods...";
	const percent = progress.percent;
	if (typeof percent === "number" && Number.isFinite(percent) && percent > 0) {
		return `${Math.min(100, Math.round(percent))}%`;
	}
	if (progress.message && progress.message.trim() !== "") return progress.message;
	return "Downloading from Nexus Mods...";
}

export function formatNexusInstallError(error: unknown): string {
	const raw = formatWailsError(error);
	if (raw.includes("no API key")) {
		return "Connect a Nexus Mods API key in Settings first.";
	}
	if (raw.includes("API key is invalid")) {
		return "That API key was rejected. Paste a new key in Settings.";
	}
	if (raw.includes("rate limited")) {
		return "Nexus Mods rate limit reached. Try again later.";
	}
	if (raw.includes("expired")) {
		return "That download link has expired. Click Mod Manager Download again.";
	}
	if (raw.includes("premium is required")) {
		return "A Premium account is required to download this file from the API. Click Mod Manager Download on the Nexus page instead.";
	}
	if (raw.includes("not for Marvel Rivals")) {
		return "Cratebug only installs Marvel Rivals mods.";
	}
	if (raw.includes("not found")) {
		return "Nexus Mods could not find that mod or file.";
	}
	return raw;
}
