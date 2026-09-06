import { useCallback, useEffect, useRef, useState } from "react";
import type { discovery } from "../../wailsjs/go/models";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type EncryptionRequiredDialogProps = {
	entries: discovery.Entry[];
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
};

const confirmDelaySeconds = 3;

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
}: EncryptionRequiredDialogProps) {
	const [secondsRemaining, setSecondsRemaining] = useState(confirmDelaySeconds);
	const ready = secondsRemaining <= 0;
	const cancelRef = useRef<HTMLButtonElement>(null);
	const names = entries.map((entry) => entry.displayName);
	const listed =
		names.length <= 8 ? names.join(", ") : `${names.slice(0, 8).join(", ")}, and more`;
	const modsLabel = entries.length === 1 ? "1 installed mod" : `${entries.length} installed mods`;

	useEffect(() => {
		if (secondsRemaining <= 0) return;
		const timeout = window.setTimeout(
			() => setSecondsRemaining((current) => current - 1),
			1000,
		);
		return () => window.clearTimeout(timeout);
	}, [secondsRemaining]);

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
				aria-labelledby="encryption-required-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="encryption-required-dialog-title">Mods need encryption</h2>
					<p className="mutation-dialog-subtitle">{modsLabel}</p>
				</div>
				<p className="delete-confirm-summary">
					Mods that change files outside <code>/Game/Marvel/Characters</code> will not
					load unless the IoStore container is encrypted with the game key. {listed}{" "}
					{entries.length === 1 ? "needs" : "need"} encryption.
				</p>
				<p className="delete-confirm-warning" role="alert">
					This rebuilds each complete IoStore bundle. Large mods can take several minutes.
					Close Marvel Rivals first. A failed rebuild leaves that mod as it was.
				</p>
				<div className="mutation-dialog-actions">
					<button
						ref={cancelRef}
						type="button"
						className="quiet-button"
						disabled={isMutating}
						onClick={onClose}
					>
						Not now
					</button>
					<button
						type="button"
						disabled={!ready || isMutating}
						onClick={() => void handleConfirm()}
					>
						{isMutating
							? "Encrypting..."
							: ready
								? "Encrypt"
								: `Encrypt (${secondsRemaining})`}
					</button>
				</div>
			</section>
		</div>
	);
}
