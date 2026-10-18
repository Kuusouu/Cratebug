import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { useEffect, useRef } from "react";
import type { gamedetect } from "../../wailsjs/go/models";
import { type LibraryProvider, libraryProviderLabels } from "./libraryTypes";
import { providerLogos } from "./StoreLogos";

type DetectLibraryDialogProps = {
	provider: LibraryProvider;
	mode: "apply" | "create";
	detection: gamedetect.Detection;
	isWorking: boolean;
	onApply: () => void;
	onCreate: () => void;
	onClose: () => void;
};

// Paths are too long to sit inline: they render as a code chip that wraps
// inside the dialog width.
const pathCodeClass =
	"my-1.5 block max-w-full rounded-md border border-border bg-secondary px-[0.6rem] py-[0.45rem] text-left text-[0.8rem] wrap-anywhere";

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
	const Logo = providerLogos[provider];
	const cancelButtonRef = useRef<HTMLButtonElement>(null);
	const primaryButtonRef = useRef<HTMLButtonElement>(null);

	// isWorking disables the focused button mid-flight, which drops focus to
	// the page; bring it back to whichever action is still enabled.
	useEffect(() => {
		const primary = primaryButtonRef.current;
		if (primary && !primary.disabled) {
			primary.focus();
			return;
		}
		const cancel = cancelButtonRef.current;
		if (cancel && !cancel.disabled) {
			cancel.focus();
		}
	}, [isWorking, mode]);

	return (
		<Dialog
			open
			onOpenChange={(nextOpen) => {
				// The Go side is mid-creation; closing now would hide the outcome
				// without stopping it, the same reason Escape stays closed during an
				// update download.
				if (!nextOpen && !isWorking) onClose();
			}}
			disablePointerDismissal
		>
			<DialogContent initialFocus={() => primaryButtonRef.current}>
				<div className="flex flex-col gap-3">
					<p className="eyebrow">
						<Logo className="size-4 flex-none align-[-0.15em]" /> {label}
					</p>
					{mode === "apply" ? (
						<>
							<DialogTitle>Found your Marvel Rivals library</DialogTitle>
							<p>
								Your {label} library already exists at{" "}
								<code className={pathCodeClass}>{detection.libraryPath}</code>
							</p>
							<p>Use it as your mod library?</p>
						</>
					) : (
						<>
							<DialogTitle>No mod library was found for {label}</DialogTitle>
							<p>
								Cratebug found Marvel Rivals in your {label} installation at{" "}
								<code className={pathCodeClass}>{detection.paksPath}</code>
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
				<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
					<Button
						size="action"
						ref={cancelButtonRef}
						type="button"
						variant="secondary"
						onClick={onClose}
						disabled={isWorking}
					>
						Cancel
					</Button>
					{mode === "apply" ? (
						<Button
							size="action"
							ref={primaryButtonRef}
							type="button"
							onClick={onApply}
							disabled={isWorking}
						>
							Use this library
						</Button>
					) : (
						<Button
							size="action"
							ref={primaryButtonRef}
							type="button"
							onClick={onCreate}
							disabled={isWorking}
						>
							{isWorking ? "Creating..." : "Create library"}
						</Button>
					)}
				</div>
			</DialogContent>
		</Dialog>
	);
}
