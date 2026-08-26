import { describe, expect, it } from "bun:test";
import { discovery, install } from "../../wailsjs/go/models";
import {
	detectLibraryCollision,
	extractModStem,
	findBatchCollisions,
	formatBytes,
	hasBlockingIssues,
	hasUnresolvedCollisions,
	validateInstallModName,
} from "./installPresentation";

describe("formatBytes", () => {
	it("handles 0 or negative bytes", () => {
		expect(formatBytes(0)).toBe("0 B");
		expect(formatBytes(-10)).toBe("0 B");
	});

	it("formats bytes, kilobytes, megabytes, and gigabytes", () => {
		expect(formatBytes(500)).toBe("500 B");
		expect(formatBytes(1024)).toBe("1.0 KB");
		expect(formatBytes(1536)).toBe("1.5 KB");
		expect(formatBytes(1048576)).toBe("1.0 MB");
		expect(formatBytes(10485760)).toBe("10.0 MB");
		expect(formatBytes(1073741824)).toBe("1.0 GB");
	});
});

describe("validateInstallModName", () => {
	it("accepts valid names", () => {
		expect(validateInstallModName("Hulk")).toBeNull();
		expect(validateInstallModName("Spider-Man Classic")).toBeNull();
		expect(validateInstallModName("Mod_9999999_P")).toBeNull();
	});

	it("rejects empty or whitespace-only names", () => {
		expect(validateInstallModName("")).toBe("Name cannot be empty.");
		expect(validateInstallModName("   ")).toBe("Name cannot be empty.");
	});

	it("rejects names ending with trailing space or period", () => {
		expect(validateInstallModName("Hulk ")).toBe("Name cannot end with a space or period.");
		expect(validateInstallModName("Hulk.")).toBe("Name cannot end with a space or period.");
	});

	it("rejects names containing Windows-reserved characters", () => {
		for (const char of ["<", ">", ":", '"', "/", "\\", "|", "?", "*"]) {
			expect(validateInstallModName(`Mod${char}Name`)).toBe(
				"Name contains a Windows-reserved character.",
			);
		}
	});

	it("rejects names containing control characters", () => {
		expect(validateInstallModName("Mod\x00Name")).toBe(
			"Name contains a Windows-reserved character.",
		);
		expect(validateInstallModName("Mod\nName")).toBe(
			"Name contains a Windows-reserved character.",
		);
	});
});

describe("hasUnresolvedCollisions", () => {
	it("returns false when there are no collisions", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			modName: "IronMan",
			collision: new install.CollisionInfo({ hasCollision: false }),
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "IronMan",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasUnresolvedCollisions([item], configs)).toBe(false);
	});

	it("returns true when a collision exists and overwrite is false", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			modName: "IronMan",
			collision: new install.CollisionInfo({ hasCollision: true }),
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "IronMan",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasUnresolvedCollisions([item], configs)).toBe(true);
	});

	it("returns false when a collision exists but overwrite is true", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			modName: "IronMan",
			collision: new install.CollisionInfo({ hasCollision: true }),
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "IronMan",
				destinationFolder: "",
				overwrite: true,
			},
		};

		expect(hasUnresolvedCollisions([item], configs)).toBe(false);
	});

	it("returns false when a collision exists on an unselected mod", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			modName: "IronMan",
			collision: new install.CollisionInfo({ hasCollision: true }),
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: false,
				modName: "IronMan",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasUnresolvedCollisions([item], configs)).toBe(false);
	});
});

describe("findBatchCollisions", () => {
	it("returns empty when all mods have distinct destinations or names", () => {
		const items = [
			new install.PreviewItem({ id: "mod-1", modName: "Hulk" }),
			new install.PreviewItem({ id: "mod-2", modName: "Thor" }),
		];
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Hulk",
				destinationFolder: "Heroes",
				overwrite: false,
			},
			"mod-2": {
				id: "mod-2",
				selected: true,
				modName: "Thor",
				destinationFolder: "Heroes",
				overwrite: false,
			},
		};

		expect(findBatchCollisions(items, configs)).toEqual({});
	});

	it("flags items when two selected mods target the exact same folder and name", () => {
		const items = [
			new install.PreviewItem({ id: "mod-1", modName: "Lagoona" }),
			new install.PreviewItem({ id: "mod-2", modName: "Lagoona" }),
		];
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Lagoona",
				destinationFolder: "Characters/InvisibleWoman",
				overwrite: false,
			},
			"mod-2": {
				id: "mod-2",
				selected: true,
				modName: "Lagoona",
				destinationFolder: "Characters/InvisibleWoman",
				overwrite: false,
			},
		};

		const result = findBatchCollisions(items, configs);
		expect(result["mod-1"]).toBeDefined();
		expect(result["mod-2"]).toBeDefined();
	});

	it("ignores collisions if one of the colliding mods is unselected", () => {
		const items = [
			new install.PreviewItem({ id: "mod-1", modName: "Lagoona" }),
			new install.PreviewItem({ id: "mod-2", modName: "Lagoona" }),
		];
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Lagoona",
				destinationFolder: "Characters/InvisibleWoman",
				overwrite: false,
			},
			"mod-2": {
				id: "mod-2",
				selected: false,
				modName: "Lagoona",
				destinationFolder: "Characters/InvisibleWoman",
				overwrite: false,
			},
		};

		expect(findBatchCollisions(items, configs)).toEqual({});
	});

	it("ignores same names when target destination folders are different", () => {
		const items = [
			new install.PreviewItem({ id: "mod-1", modName: "Lagoona" }),
			new install.PreviewItem({ id: "mod-2", modName: "Lagoona" }),
		];
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Lagoona",
				destinationFolder: "Characters/InvisibleWoman",
				overwrite: false,
			},
			"mod-2": {
				id: "mod-2",
				selected: true,
				modName: "Lagoona",
				destinationFolder: "Install/Testing",
				overwrite: false,
			},
		};

		expect(findBatchCollisions(items, configs)).toEqual({});
	});
});

describe("extractModStem", () => {
	it("strips active and disabled mod extensions", () => {
		expect(extractModStem("Mod_9999999_P.pak")).toBe("Mod_9999999_P");
		expect(extractModStem("Characters/Invis/Mod_9999999_P.pak_crateoff")).toBe("Mod_9999999_P");
		expect(extractModStem("Mod_9999999_P.bak_bento")).toBe("Mod_9999999_P");
		expect(extractModStem("Mod_9999999_P.pak_disabled")).toBe("Mod_9999999_P");
		expect(extractModStem("Mod_9999999_P.utoc")).toBe("Mod_9999999_P");
		expect(extractModStem("Mod_9999999_P.ucas")).toBe("Mod_9999999_P");
	});
});

describe("detectLibraryCollision", () => {
	it("detects live collisions against existing library entries in the same folder", () => {
		const item = new install.PreviewItem({
			id: "staged-1",
			modName: "InvisibleWoman_Skin",
		});
		const config = {
			id: "staged-1",
			selected: true,
			modName: "InvisibleWoman_Skin",
			destinationFolder: "Characters/InvisibleWoman",
			overwrite: false,
		};
		const libraryEntries = [
			new discovery.Entry({
				id: "existing-1",
				displayName: "InvisibleWoman_Skin",
				relativeFolder: "Characters/InvisibleWoman",
				primaryPath: "Characters/InvisibleWoman/InvisibleWoman_Skin_9999999_P.pak",
			}),
		];

		const collision = detectLibraryCollision(item, config, libraryEntries);
		expect(collision.hasCollision).toBe(true);
		expect(collision.existingModID).toBe("existing-1");
		expect(collision.description).toContain("Characters/InvisibleWoman");
	});

	it("clears collision when destination folder is changed to an empty folder", () => {
		const item = new install.PreviewItem({
			id: "staged-1",
			modName: "InvisibleWoman_Skin",
		});
		const config = {
			id: "staged-1",
			selected: true,
			modName: "InvisibleWoman_Skin",
			destinationFolder: "Install/Testing",
			overwrite: false,
		};
		const libraryEntries = [
			new discovery.Entry({
				id: "existing-1",
				displayName: "InvisibleWoman_Skin",
				relativeFolder: "Characters/InvisibleWoman",
				primaryPath: "Characters/InvisibleWoman/InvisibleWoman_Skin_9999999_P.pak",
			}),
		];

		const collision = detectLibraryCollision(item, config, libraryEntries);
		expect(collision.hasCollision).toBe(false);
	});

	it("clears collision when mod name is renamed to a distinct name", () => {
		const item = new install.PreviewItem({
			id: "staged-1",
			modName: "InvisibleWoman_Skin",
		});
		const config = {
			id: "staged-1",
			selected: true,
			modName: "InvisibleWoman_Skin_CustomVariant",
			destinationFolder: "Characters/InvisibleWoman",
			overwrite: false,
		};
		const libraryEntries = [
			new discovery.Entry({
				id: "existing-1",
				displayName: "InvisibleWoman_Skin",
				relativeFolder: "Characters/InvisibleWoman",
				primaryPath: "Characters/InvisibleWoman/InvisibleWoman_Skin_9999999_P.pak",
			}),
		];

		const collision = detectLibraryCollision(item, config, libraryEntries);
		expect(collision.hasCollision).toBe(false);
	});

	it("falls back to the initial backend collision when live entries are empty and config is unchanged", () => {
		const item = new install.PreviewItem({
			id: "staged-1",
			modName: "InvisibleWoman_Skin",
			destinationFolder: "Characters/InvisibleWoman",
			collision: new install.CollisionInfo({
				hasCollision: true,
				existingModID: "existing-1",
				description: 'A mod named "InvisibleWoman_Skin" already exists.',
			}),
		});
		const config = {
			id: "staged-1",
			selected: true,
			modName: "InvisibleWoman_Skin",
			destinationFolder: "Characters/InvisibleWoman",
			overwrite: false,
		};

		const collision = detectLibraryCollision(item, config, []);
		expect(collision.hasCollision).toBe(true);
		expect(collision.existingModID).toBe("existing-1");
	});

	it("does not fall back to the initial backend collision once the config has changed", () => {
		const item = new install.PreviewItem({
			id: "staged-1",
			modName: "InvisibleWoman_Skin",
			destinationFolder: "Characters/InvisibleWoman",
			collision: new install.CollisionInfo({
				hasCollision: true,
				existingModID: "existing-1",
			}),
		});
		const config = {
			id: "staged-1",
			selected: true,
			modName: "InvisibleWoman_Skin_Renamed",
			destinationFolder: "Characters/InvisibleWoman",
			overwrite: false,
		};

		const collision = detectLibraryCollision(item, config, []);
		expect(collision.hasCollision).toBe(false);
	});
});

describe("hasBlockingIssues", () => {
	it("returns false when no selected item has issues", () => {
		const item = new install.PreviewItem({ id: "mod-1", issues: [] });
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Hulk",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasBlockingIssues([item], configs)).toBe(false);
	});

	it("returns true when a selected item has a staging issue", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			issues: [
				new discovery.Issue({ code: "missing-utoc", message: "Missing .utoc sidecar" }),
			],
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: true,
				modName: "Hulk",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasBlockingIssues([item], configs)).toBe(true);
	});

	it("returns false when the item with issues is unselected", () => {
		const item = new install.PreviewItem({
			id: "mod-1",
			issues: [
				new discovery.Issue({ code: "missing-utoc", message: "Missing .utoc sidecar" }),
			],
		});
		const configs = {
			"mod-1": {
				id: "mod-1",
				selected: false,
				modName: "Hulk",
				destinationFolder: "",
				overwrite: false,
			},
		};

		expect(hasBlockingIssues([item], configs)).toBe(false);
	});
});
