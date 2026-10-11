import type { sigbypass } from "../../wailsjs/go/models";

export const signatureBypassSourceURL =
	"https://github.com/DeathChaos25/MarvelRivalsUTOCSignatureBypass";

export type SignatureBypassAction = "install" | "remove" | "none";

// Picks the card's single action from the backend state: installed offers
// removal, a missing or partial install offers installation when this build
// carries the payload, anything else offers no action.
export function signatureBypassAction(status: sigbypass.Status): SignatureBypassAction {
	if (status.state === "installed") {
		return "remove";
	}
	if (status.state === "notInstalled" || status.state === "partial") {
		return status.payloadAvailable ? "install" : "none";
	}
	return "none";
}

// Renders the backend state as one honest status line.
export function formatSignatureBypassStatus(status: sigbypass.Status): string {
	switch (status.state) {
		case "gameNotFound":
			return "Marvel Rivals was not detected. Set up your mod library first, then try again.";
		case "notInstalled":
			return status.payloadAvailable
				? "Not installed."
				: "Not installed. This build does not include the bypass files.";
		case "installed":
			return status.gameDir ? `Installed into ${status.gameDir}.` : "Installed.";
		case "partial":
			return status.payloadAvailable
				? "Incomplete install. Install again to repair it."
				: "Incomplete install. This build does not include the bypass files.";
		case "conflict":
			return describeSignatureBypassConflict(status.files);
		default:
			return "The bypass state could not be read.";
	}
}

// Names the files Cratebug refuses to touch instead of guessing at them.
export function describeSignatureBypassConflict(files?: sigbypass.FileStatus[]): string {
	const paths = (files ?? [])
		.filter((file) => file.state === "different")
		.map((file) => file.relativePath);
	if (paths.length === 0) {
		return "Different files occupy the install paths. Move them aside first, then try again.";
	}
	return `Different files already exist at ${paths.join(", ")}. Move them aside first, then try again.`;
}
