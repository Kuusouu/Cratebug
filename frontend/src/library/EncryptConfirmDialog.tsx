import { useCallback, useEffect, useRef } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { useConfirmDelay } from "./useConfirmDelay";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type EncryptConfirmDialogProps = {
	encrypt: boolean;
	entries: discovery.Entry[];
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
	skipConfirmDelay: boolean;
};

/**
 * Warns that encrypt/decrypt rebuilds each IoStore bundle instead of flipping
 * a bit, then gates confirm behind the same short delay as delete.
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

	const handleEscape = useCallback(() => {
		if (!isMutating) onClose();
	}, [isMutating, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);

	async function handleConfirm() {
		if (await onConfirm()) onClose();
	}

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="encrypt-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="encrypt-dialog-title">{action} mods</h2>
					<p className="mutation-dialog-subtitle">
						{entries.length === 1 ? names[0] : `${entries.length} mods`}
					</p>
				</div>
				<p className="delete-confirm-summary">
					Rebuilds {listed} by extracting and recreating each complete IoStore bundle.
					This is not a bit-flip. Large mods can take several minutes. Disabled primaries
					keep their disabled filename.
				</p>
				<p className="delete-confirm-warning" role="alert">
					A failed rebuild leaves that mod as it was. Already-finished mods in this batch
					stay {encrypt ? "encrypted" : "decrypted"}.
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
					</button>
				</div>
			</section>
		</div>
	);
}
