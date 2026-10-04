import { Download, Upload, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
	BackupLibrary,
	CanRetryRestore,
	CancelBackup,
	CancelRestore,
	DiscardRestorePreview,
	RestoreApply,
	RestorePreview,
} from "../../wailsjs/go/main/App";
import type { backup } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
	type BackupProgressView,
	formatBackupFileDate,
	formatBackupSummary,
	formatRestoreMetadataClause,
	formatRestoreSummary,
	formatStagedCounts,
} from "./backupPresentation";
import { formatWailsError } from "./installPresentation";
import styles from "./ToolsDialog.module.css";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

type ToolsDialogProps = {
	onClose: () => Promise<void>;
	libraryRoot: string | null;
	libraryEntryCount: number;
	onRestored: (metadataRestored: boolean) => Promise<void>;
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

// Tools dialog. Backup and Restore share one live card.
export function ToolsDialog({
	onClose,
	libraryRoot,
	libraryEntryCount,
	onRestored,
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
					</ul>
				</div>
			</section>
		</div>
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
