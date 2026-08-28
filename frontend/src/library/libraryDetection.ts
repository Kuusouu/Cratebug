import type { gamedetect } from "../../wailsjs/go/models";

// The presentation decision for one detection result: which dialog (if any)
// the frontend shows and what its confirmation action does. Kept pure so the
// three-state branching and the same-library comparison stay unit-testable
// outside the running app.
export type DetectionOutcome =
	| { kind: "apply"; libraryPath: string }
	| { kind: "create" }
	| { kind: "same-library" }
	| { kind: "not-found" };

export function detectionOutcome(
	detection: gamedetect.Detection,
	currentRoot: string,
): DetectionOutcome {
	switch (detection.state) {
		case "libraryFound": {
			const libraryPath = detection.libraryPath ?? "";
			if (!libraryPath) return { kind: "not-found" };
			if (sameWindowsPath(libraryPath, currentRoot)) return { kind: "same-library" };
			return { kind: "apply", libraryPath };
		}
		case "installFound":
			return { kind: "create" };
		default:
			return { kind: "not-found" };
	}
}

// Windows paths are case-insensitive, and the user may have set the current
// root by pasting the same library with different casing or a trailing
// separator. Uses locale-independent toLowerCase so Turkish locale (I -> ı)
// does not break the comparison - Windows case folding is invariant, matching
// Go's strings.EqualFold.
function sameWindowsPath(left: string, right: string): boolean {
	const normalize = (value: string) =>
		value
			.trim()
			.replace(/[\\/]+$/, "")
			.toLowerCase();
	return normalize(left) === normalize(right);
}
