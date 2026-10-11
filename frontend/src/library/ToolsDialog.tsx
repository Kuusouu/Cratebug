import { Download, RefreshCw, ShieldAlert, Upload, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
	BackupLibrary,
	CanRetryRestore,
	CancelBackup,
	CancelRestore,
	DiscardRestorePreview,
	InstallSignatureBypass,
	RemoveSignatureBypass,
	RestoreApply,
	RestorePreview,
	SignatureBypassStatus,
} from "../../wailsjs/go/main/App";
import type { backup, sigbypass } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import {
	type BackupProgressView,
	formatBackupFileDate,
	formatBackupSummary,
	formatRestoreMetadataClause,
	formatRestoreSummary,
	formatStagedCounts,
} from "./backupPresentation";
import { formatWailsError } from "./installPresentation";
import {
	formatSignatureBypassStatus,
	signatureBypassAction,
	signatureBypassSourceURL,
} from "./signatureBypassPresentation";
import styles from "./ToolsDialog.module.css";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type ToolsDialogProps = {
	onClose: () => Promise<void>;
	libraryRoot: string | null;
	libraryEntryCount: number;
	onRestored: (metadataRestored: boolean) => Promise<void>;
	initialBypassStatus: sigbypass.Status | null;
};

type BackupRestoreState =
	| { phase: "idle" }
	| { phase: "backing-up"; progress: BackupProgressView | null }
	| { phase: "backup-done"; summary: string }
	| { phase: "loading-preview" }
	| { phase: "preview"; preview: backup.Preview }
	| { phase: "restoring"; progress: BackupProgressView | null }
	| { phase: "restore-done"; summary: string; metadataNote: string }
	| { phase: "error"; message: string; retry: boolean };

type SignatureBypassCardState =
	| { phase: "loading" }
	| { phase: "ready"; status: sigbypass.Status }
	| { phase: "busy"; status: sigbypass.Status }
	| { phase: "error"; message: string };

// Tools dialog. Backup and Restore share one live card.
export function ToolsDialog({
	onClose,
	libraryRoot,
	libraryEntryCount,
	onRestored,
	initialBypassStatus,
}: ToolsDialogProps) {
	const closeButtonRef = useRef<HTMLButtonElement>(null);
	const cancelRequestedRef = useRef(false);
	const previewTokenRef = useRef<string | null>(null);
	const [backupRestore, setBackupRestore] = useState<BackupRestoreState>({ phase: "idle" });
	const busy =
		backupRestore.phase === "backing-up" ||
		backupRestore.phase === "loading-preview" ||
		backupRestore.phase === "restoring";

	// Keep directory handles released until an operation finishes or rolls back.
	const close = useCallback(async () => {
		if (busy) return;
		try {
			if (previewTokenRef.current) {
				const token = previewTokenRef.current;
				await DiscardRestorePreview(token);
				previewTokenRef.current = null;
			}
			await onClose();
		} catch (error) {
			setBackupRestore({ phase: "error", message: formatWailsError(error), retry: false });
		}
	}, [busy, onClose]);
	const dialogRef = useDialogFocusTrap<HTMLElement>(
		useCallback(() => {
			void close();
		}, [close]),
	);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	useEffect(() => {
		const offBackup = EventsOn("backup:progress", (progress: BackupProgressView) => {
			setBackupRestore((current) =>
				current.phase === "backing-up" ? { phase: "backing-up", progress } : current,
			);
		});
		const offRestore = EventsOn("restore:progress", (progress: BackupProgressView) => {
			setBackupRestore((current) =>
				current.phase === "restoring" ? { phase: "restoring", progress } : current,
			);
		});
		return () => {
			offBackup();
			offRestore();
		};
	}, []);

	async function startBackup() {
		if (!libraryRoot) return;
		cancelRequestedRef.current = false;
		setBackupRestore({ phase: "backing-up", progress: null });
		try {
			const result = await BackupLibrary(libraryRoot);
			if (result.cancelled || cancelRequestedRef.current) {
				setBackupRestore({ phase: "idle" });
				return;
			}
			setBackupRestore({ phase: "backup-done", summary: formatBackupSummary(result.counts) });
		} catch (error) {
			if (cancelRequestedRef.current) {
				setBackupRestore({ phase: "idle" });
				return;
			}
			setBackupRestore({ phase: "error", message: formatWailsError(error), retry: false });
		}
	}

	function cancelBackup() {
		cancelRequestedRef.current = true;
		void CancelBackup();
	}

	async function startRestorePreview() {
		if (!libraryRoot) return;
		cancelRequestedRef.current = false;
		setBackupRestore({ phase: "loading-preview" });
		try {
			if (previewTokenRef.current) {
				await DiscardRestorePreview(previewTokenRef.current);
				previewTokenRef.current = null;
			}
			const preview = await RestorePreview();
			if (preview.cancelled || cancelRequestedRef.current) {
				previewTokenRef.current = null;
				if (!preview.cancelled) {
					await DiscardRestorePreview(preview.token);
				}
				setBackupRestore({ phase: "idle" });
				return;
			}
			previewTokenRef.current = preview.token;
			setBackupRestore({ phase: "preview", preview });
		} catch (error) {
			setBackupRestore({ phase: "error", message: formatWailsError(error), retry: false });
		}
	}

	async function cancelPreview() {
		if (previewTokenRef.current) {
			const token = previewTokenRef.current;
			previewTokenRef.current = null;
			await DiscardRestorePreview(token);
		}
		setBackupRestore({ phase: "idle" });
	}

	async function confirmRestore() {
		if (!libraryRoot || !previewTokenRef.current) return;
		const token = previewTokenRef.current;
		cancelRequestedRef.current = false;
		setBackupRestore({ phase: "restoring", progress: null });
		try {
			const result = await RestoreApply(libraryRoot, token);
			if (result.cancelled) {
				setBackupRestore({ phase: "idle" });
				return;
			}
			previewTokenRef.current = null;
			await onRestored(result.metadataRestored);
			setBackupRestore({
				phase: "restore-done",
				summary: formatRestoreSummary(result.counts),
				metadataNote: result.metadataNote ?? "",
			});
		} catch (error) {
			const message = formatWailsError(error);
			try {
				const retry = await CanRetryRestore(token);
				setBackupRestore({ phase: "error", message, retry });
			} catch (retryError) {
				setBackupRestore({
					phase: "error",
					message: `${message} Could not check retry availability: ${formatWailsError(retryError)}`,
					retry: false,
				});
			}
		}
	}

	function cancelRestore() {
		cancelRequestedRef.current = true;
		void CancelRestore();
	}

	const progress =
		backupRestore.phase === "backing-up" || backupRestore.phase === "restoring"
			? backupRestore.progress
			: null;

	return (
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className={`mutation-dialog ${styles["tools-dialog"]}`}
				aria-labelledby="tools-dialog-title"
				aria-modal="true"
				role="dialog"
			>
				<div className="conflict-dialog-header">
					<div>
						<p className="eyebrow">Cratebug</p>
						<h2 id="tools-dialog-title">Tools</h2>
					</div>
					<button
						ref={closeButtonRef}
						type="button"
						className="icon-button conflict-dialog-close"
						onClick={() => void close()}
						disabled={busy}
						aria-label="Close"
					>
						<X aria-hidden="true" />
					</button>
				</div>
				<div className={[styles["tools-body"], "scroll-y"].join(" ")}>
					<ul className={styles["tools-list"]}>
						<li className={styles["tool-card"]}>
							<div className={styles["tool-card-text"]}>
								<p className={styles["tool-card-title"]}>Backup / Restore</p>
								<p className={styles["tool-card-description"]}>
									Back up the whole library to a file, or restore it from a
									backup.
								</p>
								<BackupRestoreStatus
									state={backupRestore}
									libraryEntryCount={libraryEntryCount}
								/>
							</div>
							<div className={styles["tool-card-actions"]}>
								{backupRestore.phase === "preview" ? (
									<>
										<button
											type="button"
											className="quiet-button"
											onClick={() => void confirmRestore()}
										>
											<Upload aria-hidden="true" />
											Restore
										</button>
										<button
											type="button"
											className="quiet-button"
											onClick={() => void cancelPreview()}
										>
											Cancel
										</button>
									</>
								) : backupRestore.phase === "error" && backupRestore.retry ? (
									<>
										<button
											type="button"
											className="quiet-button"
											onClick={() => void confirmRestore()}
										>
											<Upload aria-hidden="true" />
											Try again
										</button>
										<button
											type="button"
											className="quiet-button"
											onClick={() => void cancelPreview()}
										>
											Start over
										</button>
									</>
								) : busy ? (
									<button
										type="button"
										className="quiet-button"
										onClick={() =>
											backupRestore.phase === "backing-up"
												? cancelBackup()
												: cancelRestore()
										}
									>
										Cancel
									</button>
								) : (
									<>
										<button
											type="button"
											className="quiet-button"
											disabled={!libraryRoot}
											title={
												libraryRoot
													? "Back up the library"
													: "Set a mod library first"
											}
											onClick={() => void startBackup()}
										>
											<Download aria-hidden="true" />
											Backup
										</button>
										<button
											type="button"
											className="quiet-button"
											disabled={!libraryRoot}
											title={
												libraryRoot
													? "Restore from a backup"
													: "Set a mod library first"
											}
											onClick={() => void startRestorePreview()}
										>
											<Upload aria-hidden="true" />
											Restore
										</button>
									</>
								)}
							</div>
							{progress && (
								<p className={styles["tool-card-status"]} role="status">
									{backupRestore.phase === "backing-up"
										? "Backing up"
										: "Restoring"}{" "}
									{progress.current} of {progress.total}
									{progress.currentFile ? ` — ${progress.currentFile}` : ""}…
								</p>
							)}
						</li>
						<SignatureBypassCard initial={initialBypassStatus} />
					</ul>
				</div>
			</section>
		</div>
	);
}

// Starts from the status the opener read and refreshes it after every
// action. Everything here runs in event handlers; the card has no read
// effect of its own.
function SignatureBypassCard({ initial }: { initial: sigbypass.Status | null }) {
	const [card, setCard] = useState<SignatureBypassCardState>(
		initial
			? { phase: "ready", status: initial }
			: { phase: "error", message: "The bypass state could not be read." },
	);

	async function reload() {
		setCard({ phase: "loading" });
		try {
			const status = await SignatureBypassStatus();
			setCard({ phase: "ready", status });
		} catch (error) {
			setCard({ phase: "error", message: formatWailsError(error) });
		}
	}

	async function run(action: "install" | "remove") {
		const current = card.phase === "ready" || card.phase === "busy" ? card.status : null;
		if (!current) return;
		setCard({ phase: "busy", status: current });
		try {
			const status =
				action === "install"
					? await InstallSignatureBypass()
					: await RemoveSignatureBypass();
			setCard({ phase: "ready", status });
		} catch (error) {
			setCard({ phase: "error", message: formatWailsError(error) });
		}
	}

	const status = card.phase === "ready" || card.phase === "busy" ? card.status : null;
	const action = status ? signatureBypassAction(status) : "none";
	const active = action === "install" || action === "remove";

	const statusText =
		card.phase === "loading"
			? "Reading the bypass state…"
			: status
				? formatSignatureBypassStatus(status)
				: "The bypass state could not be read.";

	return (
		<li className={styles["tool-card"]}>
			<div className={styles["tool-card-text"]}>
				<p className={styles["tool-card-title"]}>Signature bypass</p>
				<p className={styles["tool-card-description"]}>Required for mods to load.</p>
				<button
					type="button"
					className="quiet-button"
					onClick={() => BrowserOpenURL(signatureBypassSourceURL)}
				>
					Source and license
				</button>
				{card.phase === "error" ? (
					<p className="mutation-dialog-error" role="alert">
						{card.message}
					</p>
				) : (
					<p className={styles["tool-card-status"]} role="status">
						{statusText}
					</p>
				)}
			</div>
			<div className={styles["tool-card-actions"]}>
				{card.phase === "error" ? (
					<button type="button" className="quiet-button" onClick={() => void reload()}>
						<RefreshCw aria-hidden="true" />
						Try again
					</button>
				) : (
					active && (
						<button
							type="button"
							className="quiet-button"
							disabled={card.phase !== "ready"}
							onClick={() => void run(action)}
						>
							<ShieldAlert aria-hidden="true" />
							{action === "install"
								? card.phase === "busy"
									? "Installing…"
									: "Install"
								: card.phase === "busy"
									? "Removing…"
									: "Remove"}
						</button>
					)
				)}
			</div>
		</li>
	);
}

function BackupRestoreStatus({
	state,
	libraryEntryCount,
}: {
	state: BackupRestoreState;
	libraryEntryCount: number;
}) {
	switch (state.phase) {
		case "idle":
			return (
				<p className={styles["tool-card-status"]} role="status">
					No backup or restore running.
				</p>
			);
		case "backing-up":
		case "restoring":
			return null;
		case "loading-preview":
			return (
				<p className={styles["tool-card-status"]} role="status">
					Reading backup file…
				</p>
			);
		case "backup-done":
			return (
				<p className={styles["tool-card-status"]} role="status">
					{state.summary}
				</p>
			);
		case "preview":
			return (
				<>
					<p className={styles["tool-card-status"]} role="status">
						{formatBackupFileDate(state.preview.zipModified)}
					</p>
					<p className={styles["tool-card-status"]} role="status">
						{formatStagedCounts(state.preview.counts)}
					</p>
					<p className={styles["tool-card-status"]} role="status">
						{formatRestoreMetadataClause(state.preview.metadataPresent)}
					</p>
					{libraryEntryCount > 0 && (
						<p className="mutation-dialog-error" role="alert">
							This will replace the {libraryEntryCount}{" "}
							{libraryEntryCount === 1 ? "mod" : "mods"} currently in the library.
							Only continue if that is what you want.
						</p>
					)}
				</>
			);
		case "restore-done":
			return (
				<>
					<p className={styles["tool-card-status"]} role="status">
						{state.summary}
					</p>
					{state.metadataNote !== "" && (
						<p className={styles["tool-card-status"]} role="status">
							{state.metadataNote}
						</p>
					)}
				</>
			);
		case "error":
			return (
				<p className="mutation-dialog-error" role="alert">
					{state.message}
				</p>
			);
	}
}
