import { EllipsisVertical } from "lucide-react";
import styles from "./BatchActionsMenu.module.css";
import { useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { EncryptionMenuState } from "./encryptionAction";
import { usePositionedPopover } from "./usePositionedPopover";

type BatchActionsMenuProps = {
	encryption: EncryptionMenuState;
	isBusy: boolean;
	selectedCount: number;
	onDelete: () => void;
	onDisable: () => void;
	onEnable: () => void;
	onEncrypt: () => void;
	onMove: () => void;
	onTags: () => void;
};

/**
 * Catalog-header dropdown for the checked set. Enable, disable, move, tags,
 * encrypt/decrypt, and delete live here. The per-mod context menu stays
 * single-target.
 */
export function BatchActionsMenu({
	encryption,
	isBusy,
	selectedCount,
	onDelete,
	onDisable,
	onEnable,
	onEncrypt,
	onMove,
	onTags,
}: BatchActionsMenuProps) {
	const triggerRef = useRef<HTMLButtonElement>(null);
	const [anchor, setAnchor] = useState<{ x: number; y: number } | null>(null);
	const open = anchor !== null;
	const empty = selectedCount === 0;
	const encryptDisabled =
		isBusy || (encryption.kind !== "encrypt" && encryption.kind !== "decrypt");

	const close = useCallback(() => {
		setAnchor(null);
	}, []);

	const { popoverRef, position } = usePositionedPopover<HTMLDivElement>(
		anchor?.x ?? 0,
		anchor?.y ?? 0,
		close,
	);

	function toggleOpen() {
		if (open) {
			close();
			return;
		}
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!rect) return;
		setAnchor({ x: rect.left, y: rect.bottom + 6 });
	}

	function choose(action: () => void) {
		close();
		action();
	}

	const encryptLabel = encryption.kind === "decrypt" ? "Decrypt" : "Encrypt";
	const container = triggerRef.current?.closest<HTMLElement>(".app-shell");

	return (
		<>
			<button
				type="button"
				ref={triggerRef}
				className={styles.trigger}
				aria-haspopup="true"
				aria-expanded={open}
				aria-label="Batch actions"
				disabled={isBusy}
				onClick={toggleOpen}
			>
				<EllipsisVertical aria-hidden="true" />
			</button>
			{open &&
				container &&
				createPortal(
					<div
						className={styles.popover}
						ref={popoverRef}
						role="menu"
						aria-label="Batch actions"
						style={{
							left: position.left,
							top: position.top,
							visibility: position.ready ? "visible" : "hidden",
						}}
					>
						<button
							type="button"
							className={styles.item}
							role="menuitem"
							disabled={isBusy || empty}
							onClick={() => choose(onEnable)}
						>
							Enable
						</button>
						<button
							type="button"
							className={styles.item}
							role="menuitem"
							disabled={isBusy || empty}
							onClick={() => choose(onDisable)}
						>
							Disable
						</button>
						<button
							type="button"
							className={styles.item}
							role="menuitem"
							disabled={isBusy || empty}
							onClick={() => choose(onMove)}
						>
							Move to...
						</button>
						<button
							type="button"
							className={styles.item}
							role="menuitem"
							disabled={isBusy || empty}
							onClick={() => choose(onTags)}
						>
							Tags...
						</button>
						<button
							type="button"
							className={styles.item}
							role="menuitem"
							disabled={encryptDisabled}
							title={encryptDisabled ? encryption.reason : undefined}
							onClick={() => choose(onEncrypt)}
						>
							{encryptLabel}
						</button>
						<button
							type="button"
							className={`${styles.item} ${styles.destructive}`}
							role="menuitem"
							disabled={isBusy || empty}
							onClick={() => choose(onDelete)}
						>
							Delete...
						</button>
					</div>,
					container,
				)}
		</>
	);
}
