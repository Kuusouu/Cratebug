import { describe, expect, it } from "bun:test";
import { gamedetect } from "../../wailsjs/go/models";
import { detectionOutcome } from "./libraryDetection";

describe("detectionOutcome", () => {
	it("offers to apply a found library", () => {
		const outcome = detectionOutcome(
			new gamedetect.Detection({
				state: "libraryFound",
				libraryPath: "C:\\Games\\MarvelRivals\\Paks\\~mods",
			}),
			"",
		);
		expect(outcome).toEqual({
			kind: "apply",
			libraryPath: "C:\\Games\\MarvelRivals\\Paks\\~mods",
		});
	});

	it("recognizes the current library regardless of case or trailing separator", () => {
		const detection = new gamedetect.Detection({
			state: "libraryFound",
			libraryPath: "C:\\Games\\MarvelRivals\\Paks\\~mods",
		});
		expect(detectionOutcome(detection, "c:\\games\\marvelrivals\\paks\\~mods\\")).toEqual({
			kind: "same-library",
		});
		expect(detectionOutcome(detection, "C:\\GAMES\\MARVELRIVALS\\PAKS\\~MODS")).toEqual({
			kind: "same-library",
		});
	});

	it("offers to create the library when only the install was found", () => {
		const outcome = detectionOutcome(
			new gamedetect.Detection({
				state: "installFound",
				paksPath: "C:\\Games\\MarvelRivals\\Paks",
			}),
			"",
		);
		expect(outcome).toEqual({ kind: "create" });
	});

	it("reports not-found, including for a malformed result", () => {
		expect(detectionOutcome(new gamedetect.Detection({ state: "notFound" }), "")).toEqual({
			kind: "not-found",
		});
		expect(detectionOutcome(new gamedetect.Detection({ state: "libraryFound" }), "")).toEqual({
			kind: "not-found",
		});
	});
});
