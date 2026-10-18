import { Button } from "@/components/ui/button";
import { useEffect, useRef } from "react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { discovery } from "../../wailsjs/go/models";
import { useConfirmDelay } from "./useConfirmDelay";

type EncryptConfirmDialogProps = {
	encrypt: boolean;
	entries: discovery.Entry[];
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
	skipConfirmDelay: boolean;
};

/**
 * Uses the same confirmation delay as delete because both actions replace archive files.
 */
export function EncryptConfirmDialog({
	encrypt,
	entries,
	isMutating,
	onClose,
	onConfirm,
	skipConfirmDelay,
}: EncryptConfirmDialogProps) {
	const { secondsRemaining, ready } = useConfirmDelay(skipConfirmDelay);
	const cancelRef = useRef<HTMLButtonElement>(null);
	const action = encrypt ? "Encrypt" : "Decrypt";
	const names = entries.map((entry) => entry.displayName);
	const listed =
		names.length <= 8 ? names.join(", ") : `${names.slice(0, 8).join(", ")}, and more`;

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
					<DialogTitle>{action} mods</DialogTitle>
					<DialogDescription className="wrap-anywhere">
						{entries.length === 1 ? names[0] : `${entries.length} mods`}
					</DialogDescription>
				</DialogHeader>
				<p className="text-[0.85rem] leading-normal text-muted-foreground">
					{encrypt
						? `Encrypts ${listed} by rewriting each complete IoStore container in place.`
						: `Decrypts ${listed} while preserving each container's identity and cooked assets.`}
				</p>
				<p className="text-[0.8rem] leading-snug font-semibold text-warn" role="status">
					{encrypt
						? "A failed encryption leaves that mod as it was."
						: "A failed decryption leaves that mod as it was."}
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
						disabled={!ready || isMutating}
						onClick={() => void handleConfirm()}
					>
						{isMutating
							? encrypt
								? "Encrypting..."
								: "Decrypting..."
							: ready
								? action
								: `${action} (${secondsRemaining})`}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
