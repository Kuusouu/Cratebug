import { useCallback, useEffect, useRef } from "react";
import styles from "./NexusLinkDialog.module.css";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";
import { useDialogFocusTrap } from "./useDialogFocusTrap";
import { formatNexusFileSize, formatNexusLinkTitle, nexusModPageURL } from "./nexusPresentation";

type NexusLinkDialogProps = {
	link: main.NexusLink;
	libraryReady: boolean;
	onConfirm: () => void;
	onCancel: () => void;
	onOpenSettings: () => void;
};

export function NexusLinkDialog({
	link,
	libraryReady,
	onConfirm,
	onCancel,
	onOpenSettings,
}: NexusLinkDialogProps) {
	const confirmRef = useRef<HTMLButtonElement>(null);
	const handleEscape = useCallback(() => onCancel(), [onCancel]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(handleEscape);
	const title = formatNexusLinkTitle(link);
	const hasFile = (link.fileId ?? 0) > 0;
	const needsAccount = !link.modName && !link.fileName;
	const canInstall = libraryReady && hasFile && !link.needsWebsite && !needsAccount;

	useEffect(() => {
		confirmRef.current?.focus();
	}, []);

	const sizeLabel = link.sizeKb && link.sizeKb > 0 ? formatNexusFileSize(link.sizeKb) : null;
	const details = [link.fileName, link.version, sizeLabel, link.author]
		.filter((part): part is string => Boolean(part && part.trim() !== ""))
		.join(" · ");

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className="mutation-dialog"
				aria-labelledby="nexus-link-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div>
					<p className="eyebrow">Nexus Mods</p>
					<h2 id="nexus-link-dialog-title">Install this download?</h2>
				</div>
				<div className={styles["nexus-link-summary"]}>
					<p className={styles["nexus-link-title"]}>{title}</p>
					{details ? <p className={styles["nexus-link-meta"]}>{details}</p> : null}
				</div>
				{needsAccount ? (
					<p className={styles["nexus-link-copy"]}>
						Connect a Nexus Mods API key in Settings to confirm this download.
					</p>
				) : null}
				{link.needsWebsite ? (
					<p className={styles["nexus-link-copy"]}>
						This link still needs a Mod Manager Download click on the Nexus website.
					</p>
				) : null}
				{!libraryReady ? (
					<p className={styles["nexus-link-copy"]}>
						Set a mod library folder before installing.
					</p>
				) : null}
				{!hasFile ? (
					<p className="mutation-dialog-error" role="alert">
						That Nexus link is missing a file.
					</p>
				) : null}
				<div className="mutation-dialog-actions">
					<button type="button" className="quiet-button" onClick={onCancel}>
						Cancel
					</button>
					{needsAccount ? (
						<button type="button" onClick={onOpenSettings}>
							Open Settings
						</button>
					) : null}
					{link.needsWebsite && link.modId ? (
						<button
							type="button"
							onClick={() =>
								BrowserOpenURL(nexusModPageURL(link.modId ?? 0, link.fileId))
							}
						>
							Open Nexus page
						</button>
					) : null}
					<button
						ref={confirmRef}
						type="button"
						disabled={!canInstall}
						onClick={onConfirm}
					>
						Download &amp; install
					</button>
				</div>
			</section>
		</div>
	);
}
