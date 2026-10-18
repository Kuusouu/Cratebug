import { expect, test } from "bun:test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

test("the installed lint rule rejects raw colors and accepts tokens, data, and selectors", () => {
	// Arrange
	const directory = mkdtempSync(join(tmpdir(), "cratebug-style-lint-"));
	const frontend = resolve(import.meta.dir, "..");
	const configPath = join(directory, "config.json");
	const fixturePath = join(directory, "fixture.tsx");
	writeFileSync(
		configPath,
		JSON.stringify({
			categories: { correctness: "off", suspicious: "off" },
			jsPlugins: [{ name: "cratebug", specifier: join(frontend, "lint/style-tokens.js") }],
			rules: { "cratebug/no-raw-tailwind-colors": "error" },
		}),
	);
	const invalid = [
		'const a = <div className="hover:bg-[#ff0000]" />;',
		'const b = cn("text-[oklch(0.5_0.2_20)]");',
		'const c = cva("block", { variants: { tone: { red: "shadow-[0_0_2px_rgba(0,0,0,0.2)]" } } });',
		'const d = `flex bg-[color-mix(in_oklch,var(--page),white_5%)] ${active ? "block" : "hidden"}`;',
		'const e = active ? "[color:hsl(0_100%_50%)]" : "text-foreground";',
	];
	const valid = [
		'const a = "bg-background text-foreground hover:bg-secondary/90";',
		'const b = "w-[23rem] bg-[var(--page)] text-[color:var(--text)]";',
		'const c = "#ff0000";',
		"const d = \"[&_a[href='#fff']]:text-primary bg-[url(#fff)]\";",
		'const e = "[#fff]:text-primary";',
	];
	try {
		for (const [source, expectedCount] of [
			[invalid, invalid.length],
			[valid, 0],
		] as const) {
			writeFileSync(fixturePath, source.join("\n"));
			// Act
			const result = Bun.spawnSync({
				cmd: [
					process.execPath,
					"run",
					"lint",
					"--config",
					configPath,
					"--format",
					"json",
					fixturePath,
				],
				cwd: frontend,
				stdout: "pipe",
				stderr: "pipe",
			});
			const report = JSON.parse(result.stdout.toString()) as {
				diagnostics: { code: string }[];
			};
			// Assert
			expect(result.exitCode).toBe(expectedCount === 0 ? 0 : 1);
			expect(report.diagnostics).toHaveLength(expectedCount);
			expect(
				report.diagnostics.every(({ code }) => code === "cratebug(no-raw-tailwind-colors)"),
			).toBe(true);
		}
	} finally {
		rmSync(directory, { recursive: true });
	}
});
