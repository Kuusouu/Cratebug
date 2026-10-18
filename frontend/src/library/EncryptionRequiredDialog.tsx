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
import { useConfirmDelay } from "./useConfirmDelay";

type EncryptionRequiredDialogProps = {
	entries: discovery.Entry[];
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
	skipConfirmDelay: boolean;
};

/**
 * Warns that unencrypted IoStore mods outside /Game/Marvel/Characters will
 * not load, then offers a pooled rebuild. Confirm uses the same short delay
 * as delete.
 */
export function EncryptionRequiredDialog({
	entries,
	isMutating,
	onClose,
	onConfirm,
	skipConfirmDelay,
}: EncryptionRequiredDialogProps) {
	const { secondsRemaining, ready } = useConfirmDelay(skipConfirmDelay);
	const cancelRef = useRef<HTMLButtonElement>(null);
	const names = entries.map((entry) => entry.displayName);
	const listed =
		names.length <= 8 ? names.join(", ") : `${names.slice(0, 8).join(", ")}, and more`;
	const modsLabel = entries.length === 1 ? "1 installed mod" : `${entries.length} installed mods`;

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
				// Escape must not dismiss while the rebuild is running.
				if (isMutating) eventDetails.cancel();
				else onClose();
			}}
			disablePointerDismissal
		>
			<DialogContent>
				<DialogHeader>
					<p className="eyebrow">Mod action</p>
					<DialogTitle>Mods need encryption</DialogTitle>
					<DialogDescription>{modsLabel}</DialogDescription>
				</DialogHeader>
				<p className="text-[0.85rem] leading-normal text-muted-foreground">
					Mods that change files outside <code>/Game/Marvel/Characters</code> will not
					load unless the IoStore container is encrypted with the game key. {listed}{" "}
					{entries.length === 1 ? "needs" : "need"} encryption.
				</p>
				<p className="text-[0.8rem] leading-snug font-semibold text-warn" role="status">
					This rebuilds each complete IoStore bundle. Close Marvel Rivals first. A failed
					rebuild leaves that mod as it was.
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
							? "Encrypting..."
							: ready
								? "Encrypt"
								: `Encrypt (${secondsRemaining})`}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
