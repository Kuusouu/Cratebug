import { useCallback, useEffect, useRef } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { hasMissingSidecar } from "./entryPresentation";
import { useConfirmDelay } from "./useConfirmDelay";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type DeleteConfirmDialogProps = {
	entries: discovery.Entry[];
	isMutating: boolean;
	onClose: () => void;
	onConfirm: (entries: discovery.Entry[]) => Promise<boolean>;
	skipConfirmDelay: boolean;
};

/**
 * Sends one or more scanner-recognized bundles to the Recycle Bin after a
 * short delay gates the confirm button, matching SPEC.md's UI safeguard.
 * The backend enforces the actual safety checks. This dialog cannot bypass them.
 */
export function DeleteConfirmDialog({
	entries,
	isMutating,
	onClose,
	onConfirm,
	skipConfirmDelay,
}: DeleteConfirmDialogProps) {
	const { secondsRemaining, ready } = useConfirmDelay(skipConfirmDelay);
	const cancelRef = useRef<HTMLButtonElement>(null);

	// Cancel, not the destructive action, gets initial focus. This also puts
	// focus inside the dialog so the shared focus trap's Escape/Tab handling
	// (which listens on the dialog element and relies on the keydown bubbling
	// from whatever currently has focus) actually has something to bubble from.
	useEffect(() => {
		cancelRef.current?.focus();
	}, []);

	const entry = entries[0];
	const missingSidecar = entries.some((item) => hasMissingSidecar(item));
	const bundleFiles = entries.flatMap((item) =>
		[item.primaryPath, item.sidecars.utoc, item.sidecars.ucas]
			.filter((path): path is string => Boolean(path))
			.map((path) => path.split("/").pop() ?? path),
	);
	const names = entries.map((item) => item.displayName);
	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	async function handleConfirm() {
		if (await onConfirm(entries)) onClose();
	}

	if (!entry) return null;

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
					<h2 id="delete-dialog-title">
						{entries.length === 1 ? "Delete mod" : `Delete ${entries.length} mods`}
					</h2>
					<p className="mutation-dialog-subtitle">
						{entries.length === 1 ? entry.displayName : names.join(", ")}
					</p>
				</div>
				<p className="delete-confirm-summary">
					Sends {entries.length === 1 ? bundleFiles.join(", ") : `${entries.length} mods`}{" "}
					to the Recycle Bin. You can restore {entries.length === 1 ? "it" : "them"} from
					there until the Recycle Bin is emptied.
				</p>
				{missingSidecar && (
					<p className="delete-confirm-warning" role="alert">
						{entries.length === 1 ? "This bundle is" : "One or more bundles are"}{" "}
						missing a recognized file. Only the files listed above will be removed.
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
