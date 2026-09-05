import { describe, expect, test } from "bun:test";
import { nextCheckedIDs, remapCheckedID, retainCheckedIDs } from "./checkedSelection";

const visible = ["a", "b", "c", "d"];

describe("nextCheckedIDs", () => {
	test("a plain click replaces the set", () => {
		const { next, anchorID } = nextCheckedIDs(new Set(["a", "c"]), visible, "b", "a", {
			shiftKey: false,
			ctrlKey: false,
			metaKey: false,
		});
		expect([...next]).toEqual(["b"]);
		expect(anchorID).toBe("b");
	});

	test("a plain click on the only checked mod empties the set", () => {
		const { next, anchorID } = nextCheckedIDs(new Set(["b"]), visible, "b", "b", {
			shiftKey: false,
			ctrlKey: false,
			metaKey: false,
		});
		expect(next.size).toBe(0);
		expect(anchorID).toBeNull();
	});

	test("ctrl-click toggles without clearing the rest", () => {
		const { next } = nextCheckedIDs(new Set(["a"]), visible, "c", "a", {
			shiftKey: false,
			ctrlKey: true,
			metaKey: false,
		});
		expect([...next].sort()).toEqual(["a", "c"]);
	});

	test("shift-click adds the range from the anchor", () => {
		const { next } = nextCheckedIDs(new Set(["b"]), visible, "d", "b", {
			shiftKey: true,
			ctrlKey: false,
			metaKey: false,
		});
		expect([...next].sort()).toEqual(["b", "c", "d"]);
	});

	test("shift-ctrl-click removes the range", () => {
		const { next } = nextCheckedIDs(new Set(["a", "b", "c", "d"]), visible, "c", "a", {
			shiftKey: true,
			ctrlKey: true,
			metaKey: false,
		});
		expect([...next]).toEqual(["d"]);
	});
});

describe("remapCheckedID", () => {
	test("replaces a renamed id", () => {
		const next = remapCheckedID(new Set(["old", "keep"]), "old", "new");
		expect([...next].sort()).toEqual(["keep", "new"]);
	});
});

describe("retainCheckedIDs", () => {
	test("drops ids that left the library", () => {
		const next = retainCheckedIDs(new Set(["keep", "gone"]), new Set(["keep"]));
		expect([...next]).toEqual(["keep"]);
	});
});
