import { useCallback, useEffect, useRef, useState } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { hasMissingSidecar } from "./entryPresentation";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type DeleteConfirmDialogProps = {
	entry: discovery.Entry;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: (entry: discovery.Entry) => Promise<boolean>;
};

// SPEC.md requires a short deliberate delay before destructive confirmation.
const deleteConfirmDelaySeconds = 3;

// Sends a scanner-recognized bundle to the Recycle Bin after a short delay
// gates the confirm button, matching SPEC.md's UI safeguard requirement.
// The backend enforces the actual safety checks; this dialog cannot bypass them.
export function DeleteConfirmDialog({
	entry,
	isMutating,
	onClose,
	onConfirm,
}: DeleteConfirmDialogProps) {
	const [secondsRemaining, setSecondsRemaining] = useState(deleteConfirmDelaySeconds);
	const ready = secondsRemaining <= 0;
	const cancelRef = useRef<HTMLButtonElement>(null);

	useEffect(() => {
		if (secondsRemaining <= 0) return;
		const timeout = window.setTimeout(
			() => setSecondsRemaining((current) => current - 1),
			1000,
		);
		return () => window.clearTimeout(timeout);
	}, [secondsRemaining]);

	// Cancel, not the destructive action, gets initial focus. This also puts
	// focus inside the dialog so the shared focus trap's Escape/Tab handling
	// (which listens on the dialog element and relies on the keydown bubbling
	// from whatever currently has focus) actually has something to bubble from.
	useEffect(() => {
		cancelRef.current?.focus();
	}, []);

	const missingSidecar = hasMissingSidecar(entry);
	const bundleFiles = [entry.primaryPath, entry.sidecars.utoc, entry.sidecars.ucas]
		.filter((path): path is string => Boolean(path))
		.map((path) => path.split("/").pop() ?? path);
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	async function handleConfirm() {
		if (await onConfirm(entry)) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="delete-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="delete-dialog-title">Delete mod</h2>
					<p className="mutation-dialog-subtitle">{entry.displayName}</p>
				</div>
				<p className="delete-confirm-summary">
					Sends {bundleFiles.join(", ")} to the Recycle Bin. You can restore it from there
					until the Recycle Bin is emptied.
				</p>
				{missingSidecar && (
					<p className="delete-confirm-warning" role="alert">
						This bundle is missing a recognized file. Only the files listed above will
						be removed.
					</p>
				)}
				<div className="mutation-dialog-actions">
					<button
						ref={cancelRef}
						type="button"
						className="quiet-button"
						disabled={isMutating}
						onClick={onClose}
					>
						Cancel
					</button>
					<button
						type="button"
						className="destructive-button"
						disabled={!ready || isMutating}
						onClick={() => void handleConfirm()}
					>
						{isMutating
							? "Deleting..."
							: ready
								? "Delete"
								: `Delete (${secondsRemaining})`}
					</button>
				</div>
			</section>
		</div>
	);
}
