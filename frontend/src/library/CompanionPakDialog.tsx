import { useCallback, useEffect, useRef, useState } from "react";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type CompanionPakDialogProps = {
	count: number;
	isMutating: boolean;
	onClose: () => void;
	onConfirm: () => Promise<boolean>;
};

const confirmDelaySeconds = 3;

/**
 * Warns that leftover chunknames / patched_files entries crash anti-cheat,
 * then offers a pak-only rewrite. Confirm uses the same short delay as delete.
 */
export function CompanionPakDialog({
	count,
	isMutating,
	onClose,
	onConfirm,
}: CompanionPakDialogProps) {
	const [secondsRemaining, setSecondsRemaining] = useState(confirmDelaySeconds);
	const ready = secondsRemaining <= 0;
	const cancelRef = useRef<HTMLButtonElement>(null);
	const modsLabel = count === 1 ? "1 installed mod" : `${count} installed mods`;

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
				aria-labelledby="companion-pak-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Mod action</p>
					<h2 id="companion-pak-dialog-title">Unsupported companion PAK entries</h2>
					<p className="mutation-dialog-subtitle">{modsLabel}</p>
				</div>
				<p className="delete-confirm-summary">
					As of 3 September 2026, <code>chunknames</code> and <code>patched_files</code>{" "}
					entries in a companion .pak cause anti-cheat crashes. {modsLabel} contain one or
					both. Rewrite the affected .pak file(s)?
				</p>
				<p className="delete-confirm-warning" role="alert">
					The IoStore .utoc and .ucas files will not be rebuilt. A failed rewrite leaves
					that mod as it was.
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
							? "Fixing..."
							: ready
								? "Rewrite PAK files"
								: `Rewrite (${secondsRemaining})`}
					</button>
				</div>
			</section>
		</div>
	);
}
