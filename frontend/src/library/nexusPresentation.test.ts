import { describe, expect, it } from "bun:test";
import {
	defaultNexusFileId,
	formatInstallProgress,
	formatNexusFileLabel,
	formatNexusFileSize,
	formatNexusInstallError,
	formatNexusLinkTitle,
	groupNexusFiles,
	nexusInstallStep,
	nexusModPageURL,
	parseNexusModPageInput,
} from "./nexusPresentation";

const connectedPremium = {
	configured: true,
	verified: true,
	isPremium: true,
};

const connectedFree = {
	configured: true,
	verified: true,
	isPremium: false,
};

describe("parseNexusModPageInput", () => {
	it("accepts both Marvel Rivals page shapes", () => {
		expect(parseNexusModPageInput("https://www.nexusmods.com/marvelrivals/mods/123")).toEqual({
			ok: true,
			url: "https://www.nexusmods.com/marvelrivals/mods/123",
		});
		expect(
			parseNexusModPageInput("  https://nexusmods.com/games/marvelrivals/mods/123/files/9  "),
		).toEqual({
			ok: true,
			url: "https://nexusmods.com/games/marvelrivals/mods/123/files/9",
		});
	});

	it("rejects empty, unsafe, non-https, and non-Rivals URLs", () => {
		expect(parseNexusModPageInput("")).toEqual({
			ok: false,
			error: "Enter a Nexus Mods page URL.",
		});
		expect(parseNexusModPageInput('https://www.nexusmods.com/marvelrivals/mods/1"x')).toEqual({
			ok: false,
			error: "That URL contains an unsafe character.",
		});
		expect(parseNexusModPageInput("http://www.nexusmods.com/marvelrivals/mods/1")).toEqual({
			ok: false,
			error: "The URL must start with https://.",
		});
		expect(parseNexusModPageInput("https://example.com/marvelrivals/mods/1")).toEqual({
			ok: false,
			error: "Enter a Nexus Mods page URL.",
		});
		expect(parseNexusModPageInput("https://www.nexusmods.com/skyrim/mods/1")).toEqual({
			ok: false,
			error: "Cratebug only installs Marvel Rivals mods from Nexus Mods.",
		});
		expect(parseNexusModPageInput("nxm://marvelrivals/mods/1/files/2")).toEqual({
			ok: false,
			error: "Paste a Nexus Mods page URL, or click Mod Manager Download in your browser.",
		});
	});
});

describe("nexusModPageURL", () => {
	it("opens the files tab and keeps an optional file id", () => {
		expect(nexusModPageURL(12)).toBe(
			"https://www.nexusmods.com/marvelrivals/mods/12?tab=files",
		);
		expect(nexusModPageURL(12, 34)).toBe(
			"https://www.nexusmods.com/marvelrivals/mods/12?tab=files&file_id=34",
		);
	});
});

describe("nexusInstallStep", () => {
	it("asks to connect until a verified key exists", () => {
		expect(nexusInstallStep(null, null)).toBe("connect");
		expect(
			nexusInstallStep({ configured: true, verified: false, isPremium: false }, null),
		).toBe("connect");
	});

	it("walks paste, file picker, website, and ready", () => {
		expect(nexusInstallStep(connectedPremium, null)).toBe("paste");
		expect(
			nexusInstallStep(connectedPremium, {
				present: true,
				modId: 1,
				needsWebsite: false,
				files: [
					{
						fileId: 9,
						name: "Main",
						fileName: "main.zip",
						version: "1.0",
						sizeKb: 10,
						categoryName: "MAIN",
						isPrimary: true,
					},
				],
			}),
		).toBe("pick-file");
		expect(
			nexusInstallStep(connectedPremium, {
				present: true,
				modId: 1,
				needsWebsite: false,
			}),
		).toBe("no-files");
		expect(
			nexusInstallStep(connectedFree, {
				present: true,
				modId: 1,
				fileId: 9,
				needsWebsite: true,
			}),
		).toBe("needs-website");
		expect(
			nexusInstallStep(connectedPremium, {
				present: true,
				modId: 1,
				fileId: 9,
				needsWebsite: false,
			}),
		).toBe("ready");
	});
});

describe("defaultNexusFileId and groupNexusFiles", () => {
	const optionalFile = {
		fileId: 2,
		name: "Optional",
		fileName: "opt.zip",
		version: "",
		sizeKb: 1,
		categoryName: "OPTIONAL",
		isPrimary: false,
	};
	const mainFile = {
		fileId: 1,
		name: "Main",
		fileName: "main.zip",
		version: "1.0",
		sizeKb: 4,
		categoryName: "MAIN",
		isPrimary: true,
	};
	const files = [optionalFile, mainFile];

	it("prefers the primary file, then the first file", () => {
		expect(defaultNexusFileId(files)).toBe(1);
		expect(defaultNexusFileId([optionalFile])).toBe(2);
		expect(defaultNexusFileId([])).toBeNull();
	});

	it("splits MAIN and OPTIONAL", () => {
		expect(groupNexusFiles(files)).toEqual({
			main: [mainFile],
			optional: [optionalFile],
		});
	});
});

describe("format helpers", () => {
	it("formats file size, label, title, and download progress", () => {
		expect(formatNexusFileSize(0)).toBe("Unknown size");
		expect(formatNexusFileSize(1024)).toBe("1.0 MB");
		expect(
			formatNexusFileLabel({
				fileId: 1,
				name: "Skin",
				fileName: "skin.zip",
				version: "2.1",
				sizeKb: 1024,
				categoryName: "MAIN",
				isPrimary: true,
			}),
		).toBe("Skin · 2.1 · 1.0 MB");
		expect(formatNexusLinkTitle({ present: true, needsWebsite: false, modName: "Cloak" })).toBe(
			"Cloak",
		);
		expect(formatNexusLinkTitle({ present: true, needsWebsite: false, modId: 44 })).toBe(
			"Mod 44",
		);
		expect(formatInstallProgress(null)).toBe("Downloading from Nexus Mods...");
		expect(
			formatInstallProgress({ percent: 41.2, message: "Downloading from Nexus Mods..." }),
		).toBe("41%");
		expect(formatInstallProgress({ message: "Starting download..." })).toBe(
			"Starting download...",
		);
	});

	it("rewrites known Nexus errors and leaves unknown text alone", () => {
		expect(formatNexusInstallError(new Error("nexus: no API key configured"))).toBe(
			"Connect a Nexus Mods API key in Settings first.",
		);
		expect(formatNexusInstallError(new Error("nexus: rate limited"))).toBe(
			"Nexus Mods rate limit reached. Try again later.",
		);
		expect(formatNexusInstallError(new Error("nexus: download link has expired"))).toBe(
			"That download link has expired. Click Mod Manager Download again.",
		);
		expect(formatNexusInstallError(new Error("disk is full"))).toBe("disk is full");
	});
});
