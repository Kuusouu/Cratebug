import { describe, expect, test } from "bun:test";
import { placeholderTools } from "./toolsPlaceholder";

describe("placeholderTools", () => {
	test("every placeholder tool has the fields the dialog renders", () => {
		const ids = new Set<string>();
		for (const tool of placeholderTools) {
			expect(tool.id.length).toBeGreaterThan(0);
			expect(tool.title.length).toBeGreaterThan(0);
			expect(tool.description.length).toBeGreaterThan(0);
			expect(tool.status.length).toBeGreaterThan(0);
			expect(tool.actionLabel.length).toBeGreaterThan(0);
			expect(ids.has(tool.id)).toBe(false);
			ids.add(tool.id);
		}
	});
});
