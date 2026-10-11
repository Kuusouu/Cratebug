import { describe, expect, test } from "bun:test";
import { sigbypass } from "../../wailsjs/go/models";
import {
	describeSignatureBypassConflict,
	formatSignatureBypassStatus,
	signatureBypassAction,
} from "./signatureBypassPresentation";

function bypassStatus(overrides: Partial<sigbypass.Status> = {}): sigbypass.Status {
	return new sigbypass.Status({
		state: "notInstalled",
		payloadAvailable: true,
		...overrides,
	});
}

describe("signatureBypassAction", () => {
	test("offers installation for a missing payload that ships with the build", () => {
		expect(signatureBypassAction(bypassStatus({ state: "notInstalled" }))).toBe("install");
	});

	test("offers installation to repair a partial one", () => {
		expect(signatureBypassAction(bypassStatus({ state: "partial" }))).toBe("install");
	});

	test("offers removal once installed", () => {
		expect(signatureBypassAction(bypassStatus({ state: "installed" }))).toBe("remove");
	});

	test("offers no action without a payload to install from", () => {
		expect(
			signatureBypassAction(bypassStatus({ state: "notInstalled", payloadAvailable: false })),
		).toBe("none");
		expect(
			signatureBypassAction(bypassStatus({ state: "partial", payloadAvailable: false })),
		).toBe("none");
	});

	test("offers no action for a conflict or a missing game", () => {
		expect(signatureBypassAction(bypassStatus({ state: "conflict" }))).toBe("none");
		expect(signatureBypassAction(bypassStatus({ state: "gameNotFound" }))).toBe("none");
	});
});

describe("formatSignatureBypassStatus", () => {
	test("states the install is missing without lecturing", () => {
		expect(formatSignatureBypassStatus(bypassStatus({ state: "notInstalled" }))).toBe(
			"Not installed.",
		);
	});

	test("admits a build that does not carry the payload", () => {
		expect(
			formatSignatureBypassStatus(
				bypassStatus({ state: "notInstalled", payloadAvailable: false }),
			),
		).toBe("Not installed. This build does not include the bypass files.");
	});

	test("names the game directory of an install", () => {
		expect(
			formatSignatureBypassStatus(
				bypassStatus({ state: "installed", gameDir: "C:\\Games\\MarvelRivals\\Win64" }),
			),
		).toBe("Installed into C:\\Games\\MarvelRivals\\Win64.");
	});

	test("sends a library-less user back to detection", () => {
		expect(formatSignatureBypassStatus(bypassStatus({ state: "gameNotFound" }))).toContain(
			"Set up your mod library first",
		);
	});

	test("uses the installer's own state words for repair", () => {
		expect(formatSignatureBypassStatus(bypassStatus({ state: "partial" }))).toContain(
			"Incomplete install",
		);
	});
});

describe("describeSignatureBypassConflict", () => {
	test("names the occupying files instead of guessing", () => {
		const rendered = describeSignatureBypassConflict([
			new sigbypass.FileStatus({ relativePath: "dsound.dll", state: "different" }),
			new sigbypass.FileStatus({
				relativePath: "plugins\\bypass.asi",
				state: "different",
			}),
			new sigbypass.FileStatus({ relativePath: "plugins\\other.asi", state: "missing" }),
		]);
		expect(rendered).toContain("dsound.dll");
		expect(rendered).toContain("plugins\\bypass.asi");
		expect(rendered).not.toContain("other.asi");
	});

	test("stays honest when the file list is missing", () => {
		expect(describeSignatureBypassConflict(undefined)).toContain("Different files occupy");
		expect(describeSignatureBypassConflict([])).toContain("Different files occupy");
	});
});
