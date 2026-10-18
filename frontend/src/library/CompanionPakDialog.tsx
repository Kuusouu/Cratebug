import { useEffect, useRef } from "react";

import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";

import { useConfirmDelay } from "./useConfirmDelay";

type CompanionPakDialogProps = {
	count: number;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
	skipConfirmDelay: boolean;
};

/**
 * Warns that leftover chunknames / patched_files entries crash anti-cheat,
 * then offers a pak-only rewrite. Confirm uses the same short delay as delete.
 */
export function CompanionPakDialog({
	count,
	isMutating,
	onClose,
	onConfirm,
	skipConfirmDelay,
}: CompanionPakDialogProps) {
	const { secondsRemaining, ready } = useConfirmDelay(skipConfirmDelay);
	const cancelRef = useRef<HTMLButtonElement>(null);
	const modsLabel = count === 1 ? "1 installed mod" : `${count} installed mods`;

	useEffect(() => {
		cancelRef.current?.focus();
	}, []);

	async function handleConfirm() {
		if (await onConfirm()) onClose();
	}

	return (
		<Dialog
			open
			onOpenChange={(nextOpen, eventDetails) => {
				if (nextOpen) return;
				// Escape must not dismiss while the rewrite is running.
				if (isMutating) eventDetails.cancel();
				else onClose();
			}}
			disablePointerDismissal
		>
			<DialogContent>
				<DialogHeader>
					<p className="eyebrow">Mod action</p>
					<DialogTitle>Unsupported companion PAK entries</DialogTitle>
					<DialogDescription>{modsLabel}</DialogDescription>
				</DialogHeader>
				<p className="text-[0.85rem] leading-normal text-muted-foreground">
					As of 3 September 2026, <code>chunknames</code> and <code>patched_files</code>{" "}
					entries in a companion .pak cause anti-cheat crashes. {modsLabel} contain one or
					both. Rewrite the affected .pak file(s)?
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
						Not now
					</Button>
					<Button
						size="action"
						type="button"
						disabled={!ready || isMutating}
						onClick={() => void handleConfirm()}
					>
						{isMutating
							? "Fixing..."
							: ready
								? "Rewrite PAK files"
								: `Rewrite (${secondsRemaining})`}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
