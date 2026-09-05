import { useCallback, useEffect, useRef, useState } from "react";
import { IsFolderEmpty } from "../../wailsjs/go/main/App";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type FolderDeleteConfirmDialogProps = {
	folder: string;
	libraryRoot: string;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: (folder: string) => Promise<boolean>;
};

// SPEC.md requires a short deliberate delay before destructive confirmation.
const deleteConfirmDelaySeconds = 3;

// Mirrors the mod delete dialog's deliberate delay. The emptiness check runs
// against the real directory listing, not the mod index, so files the scanner
// does not model still trigger the contents warning.
export function FolderDeleteConfirmDialog({
	folder,
	libraryRoot,
	isMutating,
	onClose,
	onConfirm,
}: FolderDeleteConfirmDialogProps) {
	const [secondsRemaining, setSecondsRemaining] = useState(deleteConfirmDelaySeconds);
	const [isEmpty, setIsEmpty] = useState<boolean | null>(null);
	const ready = secondsRemaining <= 0 && isEmpty !== null;
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

	useEffect(() => {
		let cancelled = false;
		IsFolderEmpty(libraryRoot, folder)
			.then((empty) => {
				if (!cancelled) setIsEmpty(empty);
			})
			.catch(() => {
				// A failed check never unlocks the weaker empty-folder wording.
				if (!cancelled) setIsEmpty(false);
			});
		return () => {
			cancelled = true;
		};
	}, [folder, libraryRoot]);

	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	const folderName = folder.split("/").at(-1) ?? folder;

	async function handleConfirm() {
		if (await onConfirm(folder)) onClose();
	}

	const summary =
		isEmpty === null
			? "Checking the folder's contents..."
			: isEmpty
				? `Sends the empty folder ${folderName} to the Recycle Bin. You can restore it from there until the Recycle Bin is emptied.`
				: `The folder ${folderName} is not empty. All of its contents will be deleted with it. The entire folder can be restored from the Recycle Bin until it is emptied.`;

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="folder-delete-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Folder action</p>
					<h2 id="folder-delete-dialog-title">Delete folder</h2>
					<p className="mutation-dialog-subtitle">{folder}</p>
				</div>
				<p
					className={
						isEmpty === false ? "delete-confirm-warning" : "delete-confirm-summary"
					}
					role={isEmpty === false ? "alert" : undefined}
				>
					{summary}
				</p>
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
							: isEmpty === null
								? "Delete..."
								: ready
									? "Delete"
									: `Delete (${secondsRemaining})`}
					</button>
				</div>
			</section>
		</div>
	);
}
