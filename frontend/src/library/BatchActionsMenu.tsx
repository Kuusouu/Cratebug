import { EllipsisVertical } from "lucide-react";
import styles from "./BatchActionsMenu.module.css";
import type { EncryptionMenuState } from "./encryptionAction";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

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
	const empty = selectedCount === 0;
	const encryptDisabled =
		isBusy || (encryption.kind !== "encrypt" && encryption.kind !== "decrypt");
	const encryptLabel = encryption.kind === "decrypt" ? "Decrypt" : "Encrypt";

	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				disabled={isBusy}
				render={
					<button type="button" className={styles.trigger} aria-label="Batch actions" />
				}
			>
				<EllipsisVertical aria-hidden="true" />
			</DropdownMenuTrigger>
			<DropdownMenuContent
				sideOffset={6}
				// Sized to the labels, not to the compact icon trigger.
				className="w-auto min-w-46"
			>
				<DropdownMenuItem disabled={isBusy || empty} onClick={onEnable}>
					Enable
				</DropdownMenuItem>
				<DropdownMenuItem disabled={isBusy || empty} onClick={onDisable}>
					Disable
				</DropdownMenuItem>
				<DropdownMenuItem disabled={isBusy || empty} onClick={onMove}>
					Move to...
				</DropdownMenuItem>
				<DropdownMenuItem disabled={isBusy || empty} onClick={onTags}>
					Tags...
				</DropdownMenuItem>
				<DropdownMenuItem
					disabled={encryptDisabled}
					title={encryptDisabled ? encryption.reason : undefined}
					onClick={onEncrypt}
				>
					{encryptLabel}
				</DropdownMenuItem>
				<DropdownMenuItem
					variant="destructive"
					disabled={isBusy || empty}
					onClick={onDelete}
				>
					Delete...
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
