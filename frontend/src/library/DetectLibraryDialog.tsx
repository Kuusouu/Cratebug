import { useCallback, useEffect, useRef } from "react";
import type { gamedetect } from "../../wailsjs/go/models";
import { type LibraryProvider, libraryProviderLabels } from "./libraryTypes";
import { SteamLogo } from "./StoreLogos";
import { focusableSelector, useDialogFocusTrap } from "./useDialogFocusTrap";

type DetectLibraryDialogProps = {
	provider: LibraryProvider;
	mode: "apply" | "create";
	detection: gamedetect.Detection;
	isWorking: boolean;
	onApply: () => void;
	onCreate: () => void;
	onClose: () => void;
};

// Shows the outcome of one auto-detection attempt that needs a decision:
// switching to a found-but-different library, or creating the missing
// ~mods folder inside a verified installation. The "not found" outcome has
// no dialog; it reports through the regular feedback toast instead.
export function DetectLibraryDialog({
	provider,
	mode,
	detection,
	isWorking,
	onApply,
	onCreate,
	onClose,
}: DetectLibraryDialogProps) {
	const label = libraryProviderLabels[provider];
	const handleEscape = useCallback(() => {
		// The Go side is mid-creation; closing now would hide the outcome
		// without stopping it, the same reason Escape stays closed during an
		// update download.
		if (!isWorking) onClose();
	}, [isWorking, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const cancelButtonRef = useRef<HTMLButtonElement>(null);
	const primaryButtonRef = useRef<HTMLButtonElement>(null);

	// biome-ignore lint/correctness/useExhaustiveDependencies: ref.current is stable; effect intentionally re-runs only on isWorking/mode for focus management
	useEffect(() => {
		const primary = primaryButtonRef.current;
		if (primary && !primary.disabled) {
			primary.focus();
			return;
		}
		const cancel = cancelButtonRef.current;
		if (cancel && !cancel.disabled) {
			cancel.focus();
			return;
		}
		const dialog = dialogRef.current;
		if (!dialog) return;
		const fallback = dialog.querySelector<HTMLElement>(focusableSelector);
		if (fallback) {
			fallback.focus();
			return;
		}
		// Both actions disabled during creation - keep focus inside dialog so
		// the Escape listener on the container remains reachable.
		dialog.focus();
	}, [isWorking, mode]);

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="detect-library-dialog-title"
				aria-modal="true"
				role="dialog"
				tabIndex={-1}
			>
				<div>
					<p className="eyebrow">
						<SteamLogo className="detect-dialog-logo" /> {label}
					</p>
					{mode === "apply" ? (
						<>
							<h2 id="detect-library-dialog-title">
								Found your Marvel Rivals library
							</h2>
							<p>
								Your {label} library already exists at{" "}
								<code className="detect-dialog-path">{detection.libraryPath}</code>
							</p>
							<p>Use it as your mod library?</p>
						</>
					) : (
						<>
							<h2 id="detect-library-dialog-title">
								No mod library was found for {label}
							</h2>
							<p>
								Cratebug found Marvel Rivals in your {label} installation at{" "}
								<code className="detect-dialog-path">{detection.paksPath}</code>
							</p>
							<p>
								The <code>~mods</code> folder it loads mods from does not exist yet.
							</p>
							<p>
								Create it? Cratebug creates only that one folder, and nothing is
								written inside it.
							</p>
						</>
					)}
				</div>
				<div className="mutation-dialog-actions">
					<button
						ref={cancelButtonRef}
						type="button"
						className="quiet-button"
						onClick={onClose}
						disabled={isWorking}
					>
						Cancel
					</button>
					{mode === "apply" ? (
						<button
							ref={primaryButtonRef}
							type="button"
							onClick={onApply}
							disabled={isWorking}
						>
							Use this library
						</button>
					) : (
						<button
							ref={primaryButtonRef}
							type="button"
							onClick={onCreate}
							disabled={isWorking}
						>
							{isWorking ? "Creating..." : "Create library"}
						</button>
					)}
				</div>
			</section>
		</div>
	);
}
