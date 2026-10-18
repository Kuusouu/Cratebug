import { useEffect, useRef } from "react";

import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";

import type { discovery } from "../../wailsjs/go/models";
import { hasMissingSidecar } from "./entryPresentation";
import { useConfirmDelay } from "./useConfirmDelay";

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

	// Cancel, not the destructive action, gets initial focus.
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

	async function handleConfirm() {
		if (await onConfirm(entries)) onClose();
	}

	if (!entry) return null;

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
					<p className="eyebrow">Mod action</p>
					<DialogTitle>
						{entries.length === 1 ? "Delete mod" : `Delete ${entries.length} mods`}
					</DialogTitle>
					<DialogDescription className="wrap-anywhere">
						{entries.length === 1 ? entry.displayName : names.join(", ")}
					</DialogDescription>
				</DialogHeader>
				<p className="text-[0.85rem] leading-normal text-muted-foreground">
					Sends {entries.length === 1 ? bundleFiles.join(", ") : `${entries.length} mods`}{" "}
					to the Recycle Bin. You can restore {entries.length === 1 ? "it" : "them"} from
					there.
				</p>
				{missingSidecar && (
					<p
						className="text-[0.8rem] leading-snug font-semibold text-destructive"
						role="alert"
					>
						{entries.length === 1 ? "This bundle is" : "One or more bundles are"}{" "}
						missing a recognized file. Only the files listed above will be removed.
					</p>
				)}
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
							: ready
								? "Delete"
								: `Delete (${secondsRemaining})`}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
