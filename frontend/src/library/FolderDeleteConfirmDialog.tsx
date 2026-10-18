import { Button } from "@/components/ui/button";
import { useEffect, useRef, useState } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { IsFolderEmpty } from "../../wailsjs/go/main/App";
import { useConfirmDelay } from "./useConfirmDelay";

type FolderDeleteConfirmDialogProps = {
	folder: string;
	libraryRoot: string;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: (folder: string) => Promise<boolean>;
	skipConfirmDelay: boolean;
};

// Mirrors the mod delete dialog's deliberate delay. The emptiness check runs
// against the real directory listing, not the mod index, so files the scanner
// does not model still trigger the contents warning.
export function FolderDeleteConfirmDialog({
	folder,
	libraryRoot,
	isMutating,
	onClose,
	onConfirm,
	skipConfirmDelay,
}: FolderDeleteConfirmDialogProps) {
	const { secondsRemaining, ready: delayReady } = useConfirmDelay(skipConfirmDelay);
	const [isEmpty, setIsEmpty] = useState<boolean | null>(null);
	const ready = delayReady && isEmpty !== null;
	const cancelRef = useRef<HTMLButtonElement>(null);

	// Cancel, not the destructive action, gets initial focus.
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

	const folderName = folder.split("/").at(-1) ?? folder;

	async function handleConfirm() {
		if (await onConfirm(folder)) onClose();
	}

	const summary =
		isEmpty === null
			? "Checking the folder's contents..."
			: isEmpty
				? `Sends the empty folder ${folderName} to the Recycle Bin. You can restore it from there.`
				: `The folder ${folderName} is not empty. All of its contents will be deleted with it. The entire folder can be restored from the Recycle Bin.`;

	return (
		<Dialog
			open
			onOpenChange={(nextOpen, eventDetails) => {
				if (nextOpen) return;
				// Escape must not dismiss while the deletion is running.
				if (isMutating) eventDetails.cancel();
				else onClose();
			}}
			disablePointerDismissal
		>
			<DialogContent>
				<DialogHeader>
					<p className="eyebrow">Folder action</p>
					<DialogTitle>Delete folder</DialogTitle>
					<DialogDescription className="wrap-anywhere">{folder}</DialogDescription>
				</DialogHeader>
				<p
					className={
						isEmpty === false
							? "text-[0.8rem] leading-snug font-semibold text-destructive"
							: "text-[0.85rem] leading-normal text-muted-foreground"
					}
					role={isEmpty === false ? "alert" : undefined}
				>
					{summary}
				</p>
				<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
					<Button
						size="action"
						ref={cancelRef}
						type="button"
						variant="secondary"
						disabled={isMutating}
						onClick={onClose}
					>
						Cancel
					</Button>
					<Button
						size="action"
						type="button"
						variant="destructiveSolid"
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
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
