import { Button } from "@/components/ui/button";
import {
	ChevronDown,
	ChevronRight,
	CircleAlert,
	Loader2,
	Package,
	PackagePlus,
	TriangleAlert,
	X,
} from "lucide-react";
import styles from "./InstallPreviewDialog.module.css";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
	ApplyInstall,
	CancelInstall,
	CancelNexusDownload,
	PrepareInstall,
	PrepareNexusInstall,
} from "../../wailsjs/go/main/App";
import { type discovery, install } from "../../wailsjs/go/models";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	categorySlug,
	entryCategoryLabel,
	entryCharacterLabel,
	entryHeroPortraitUrl,
} from "./entryPresentation";
import {
	defaultModConfig,
	detectLibraryCollision,
	findBatchCollisions,
	formatBytes,
	formatWailsError as formatError,
	hasBlockingIssues,
	hasUnresolvedCollisions,
	selectedUnsupportedCompanionCount,
	type ModConfig,
	validateInstallModName,
} from "./installPresentation";
import { type InstallProgressView, formatInstallProgress } from "./nexusPresentation";

export type InstallSource =
	| { kind: "files"; paths: string[] }
	| { kind: "nexus"; modId: number; fileId: number };

export type InstallPreviewDialogProps = {
	modRoot: string;
	source: InstallSource;
	defaultFolder: string;
	folders: string[];
	libraryEntries?: readonly discovery.Entry[];
	downloadProgress?: InstallProgressView | null;
	onDone: (result: install.ApplyResult) => void;
	onCancel: () => void;
};

type DialogPhase = "preparing" | "ready" | "applying" | "error";

// Base UI treats "" as "no selection", so the library-root option carries a
// sentinel that is translated back to "" before it reaches the install calls.
const LIBRARY_ROOT_OPTION = "__cratebug_library_root__";

export function InstallPreviewDialog({
	modRoot,
	source,
	defaultFolder,
	folders,
	libraryEntries = [],
	downloadProgress = null,
	onDone,
	onCancel,
}: InstallPreviewDialogProps) {
	const [phase, setPhase] = useState<DialogPhase>("preparing");
	const [errorMessage, setErrorMessage] = useState("");
	const [previewResult, setPreviewResult] = useState<install.PreviewResult | null>(null);
	const [configs, setConfigs] = useState<Record<string, ModConfig>>({});
	const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});
	const sessionIdRef = useRef<string | null>(null);
	const closeButtonRef = useRef<HTMLButtonElement>(null);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	// Cancel the staging session when the dialog closes (Escape or close button).
	const handleCancel = useCallback(async () => {
		if (source.kind === "nexus") {
			try {
				await CancelNexusDownload();
			} catch {
				// Best-effort cleanup
			}
		}
		const session = sessionIdRef.current;
		if (session) {
			try {
				await CancelInstall(session);
			} catch {
				// Best-effort cleanup
			}
		}
		onCancel();
	}, [onCancel, source.kind]);

	// Stage files and generate preview on mount
	useEffect(() => {
		let isMounted = true;

		async function prepare() {
			setPhase("preparing");
			setErrorMessage("");
			try {
				const result =
					source.kind === "nexus"
						? await PrepareNexusInstall(
								modRoot,
								source.modId,
								source.fileId,
								defaultFolder,
							)
						: await PrepareInstall(modRoot, source.paths, defaultFolder);
				sessionIdRef.current = result.sessionId;

				if (!isMounted) {
					// The dialog was cancelled while this request was still in flight.
					// handleCancel ran before sessionIdRef had a value to clean up, so
					// finish that cleanup here instead of leaking the staging session.
					try {
						await CancelInstall(result.sessionId);
					} catch {
						// Best-effort cleanup
					}
					return;
				}

				setPreviewResult(result);

				const initialConfigs: Record<string, ModConfig> = {};
				for (const item of result.items) {
					initialConfigs[item.id] = defaultModConfig(item);
				}
				setConfigs(initialConfigs);
				setPhase("ready");
			} catch (error) {
				if (!isMounted) return;
				setErrorMessage(formatError(error));
				setPhase("error");
			}
		}

		void prepare();

		return () => {
			isMounted = false;
		};
	}, [modRoot, source, defaultFolder]);

	const items = useMemo(() => previewResult?.items ?? [], [previewResult?.items]);

	// Partition and sort items: selected first, unselected at bottom
	const sortedItems = useMemo(() => {
		return [...items].sort((a, b) => {
			const aSelected = configs[a.id]?.selected ?? true;
			const bSelected = configs[b.id]?.selected ?? true;
			if (aSelected === bSelected) return 0;
			return aSelected ? -1 : 1;
		});
	}, [items, configs]);

	const selectedItems = useMemo(() => {
		return items.filter((item) => configs[item.id]?.selected ?? true);
	}, [items, configs]);

	// Validation checks across only selected items
	const validationErrors = useMemo(() => {
		const errors: Record<string, string> = {};
		for (const item of selectedItems) {
			const config = configs[item.id];
			if (!config) continue;

			const nameError = validateInstallModName(config.modName);
			if (nameError) {
				errors[item.id] = nameError;
			}
		}
		return errors;
	}, [selectedItems, configs]);

	// Check if any selected mod has an unresolved collision against the existing library
	const unresolvedCollisions = useMemo(() => {
		return hasUnresolvedCollisions(items, configs, libraryEntries);
	}, [items, configs, libraryEntries]);

	// Check if multiple selected mods target the exact same destination folder and name
	const batchCollisions = useMemo(() => {
		return findBatchCollisions(items, configs);
	}, [items, configs]);

	const hasBatchCollisions = Object.keys(batchCollisions).length > 0;

	// Check if any selected mod has an incomplete bundle from a staging failure
	const blockingIssues = useMemo(() => {
		return hasBlockingIssues(items, configs);
	}, [items, configs]);

	const unsupportedCompanionCount = useMemo(() => {
		return selectedUnsupportedCompanionCount(items, configs);
	}, [items, configs]);

	const canInstall =
		phase === "ready" &&
		selectedItems.length > 0 &&
		Object.keys(validationErrors).length === 0 &&
		!unresolvedCollisions &&
		!hasBatchCollisions &&
		!blockingIssues;

	const handleConfigChange = useCallback((id: string, updates: Partial<ModConfig>) => {
		setConfigs((prev) => {
			const existing = prev[id];
			if (!existing) return prev;
			return {
				...prev,
				[id]: {
					...existing,
					...updates,
				},
			};
		});
	}, []);

	const toggleFileExpansion = useCallback((id: string) => {
		setExpandedFiles((prev) => ({
			...prev,
			[id]: !(prev[id] ?? true),
		}));
	}, []);

	async function handleApply() {
		if (!previewResult || !canInstall) return;

		setPhase("applying");
		setErrorMessage("");

		try {
			const applyItems = selectedItems.map((item) => {
				const config = configs[item.id] ?? defaultModConfig(item);
				return new install.ApplyItem({
					id: item.id,
					modName: config.modName.trim(),
					destinationFolder: config.destinationFolder,
					overwrite: config.overwrite,
					encrypt: config.encrypt,
				});
			});

			const result = await ApplyInstall(modRoot, previewResult.sessionId, applyItems);
			sessionIdRef.current = null; // Cleaned up by backend apply
			onDone(result);
		} catch (error) {
			setErrorMessage(formatError(error));
			setPhase("error");
		}
	}

	const buttonLabel = useMemo(() => {
		if (selectedItems.length === items.length) {
			return items.length === 1 ? "Install mod" : `Install ${items.length} mods`;
		}
		return `Install ${selectedItems.length} of ${items.length} mods`;
	}, [selectedItems.length, items.length]);

	return (
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (!nextOpen && phase !== "applying") void handleCancel();
			}}
		>
			<DialogContent className="max-h-[min(calc(100vh_-_3.5rem),52rem)] overflow-hidden overscroll-contain sm:max-w-[56rem]">
				<div className={styles["install-preview-header"]}>
					<div>
						<p className="eyebrow">Mod installation</p>
						<DialogTitle>
							{phase === "preparing"
								? "Inspecting files..."
								: items.length === 1
									? "Install 1 mod"
									: `Install ${items.length} mods`}
						</DialogTitle>
					</div>
					{phase !== "applying" && (
						<Button
							ref={closeButtonRef}
							type="button"
							variant="secondary"
							size="icon"
							onClick={() => void handleCancel()}
							aria-label="Close"
							title="Cancel installation"
						>
							<X aria-hidden="true" />
						</Button>
					)}
				</div>

				{phase === "preparing" && (
					<div className={styles["install-preview-status-state"]}>
						<Loader2 className="spinning-loader" aria-hidden="true" />
						<p>
							{source.kind === "nexus"
								? formatInstallProgress(downloadProgress)
								: "Extracting archives and discovering mod bundles..."}
						</p>
						{source.kind === "nexus" ? (
							<div
								className={styles["install-download-progress"]}
								role="progressbar"
								aria-valuemin={0}
								aria-valuemax={100}
								aria-valuenow={
									typeof downloadProgress?.percent === "number"
										? Math.min(100, Math.round(downloadProgress.percent))
										: undefined
								}
							>
								<div
									className={styles["install-download-progress-fill"]}
									style={{
										width:
											typeof downloadProgress?.percent === "number"
												? `${Math.min(100, Math.max(0, downloadProgress.percent))}%`
												: "15%",
									}}
								/>
							</div>
						) : null}
					</div>
				)}

				{phase === "error" && (
					<div
						className={[styles["install-preview-status-state"], styles.error].join(" ")}
					>
						<CircleAlert aria-hidden="true" />
						<p>{errorMessage || "An error occurred during installation."}</p>
						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={() => void handleCancel()}
							>
								Close
							</Button>
						</div>
					</div>
				)}

				{(phase === "ready" || phase === "applying") && items.length === 0 && (
					<div className={styles["install-preview-status-state"]}>
						<Package aria-hidden="true" />
						<p>No installable mod files were found in the selected files.</p>
						<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
							<Button
								size="action"
								type="button"
								variant="secondary"
								onClick={() => void handleCancel()}
							>
								Close
							</Button>
						</div>
					</div>
				)}

				{(phase === "ready" || phase === "applying") && items.length > 0 && (
					<>
						<div className={[styles["install-preview-list"], "scroll-y"].join(" ")}>
							{sortedItems.map((item) => {
								const config = configs[item.id] ?? defaultModConfig(item);
								const isSelected = config.selected;
								const isIoStore = item.bundleFormat === "iostore";
								const isExpanded = expandedFiles[item.id] ?? true;
								const error = isSelected ? validationErrors[item.id] : undefined;
								const liveCollision = detectLibraryCollision(
									item,
									config,
									libraryEntries,
								);
								const hasCollision = isSelected && liveCollision.hasCollision;
								const categoryLabel = entryCategoryLabel(item.identity);
								const characterLabel = entryCharacterLabel(item.identity);
								const heroPortraitUrl = entryHeroPortraitUrl(item.identity);

								return (
									<div
										key={item.id}
										className={[
											styles["install-mod-card"],
											!isSelected ? styles.unselected : "",
										]
											.filter(Boolean)
											.join(" ")}
									>
										<div className={styles["install-mod-card-header"]}>
											<div className={styles["install-mod-info"]}>
												<div className={styles["install-mod-select-hero"]}>
													<label
														className={
															styles["install-mod-select-label"]
														}
													>
														<input
															type="checkbox"
															checked={isSelected}
															disabled={phase === "applying"}
															onChange={(event) =>
																handleConfigChange(item.id, {
																	selected: event.target.checked,
																})
															}
														/>
														<span
															className={
																styles["install-mod-select-text"]
															}
														>
															{isSelected
																? "Include in install"
																: "Excluded"}
														</span>
													</label>

													{characterLabel && (
														<div
															className={styles["install-hero-pill"]}
															title={characterLabel}
														>
															<div
																className={
																	styles["install-hero-thumbnail"]
																}
															>
																{heroPortraitUrl ? (
																	<img
																		src={heroPortraitUrl}
																		alt=""
																		className="mod-thumbnail-hero"
																	/>
																) : (
																	<Package aria-hidden="true" />
																)}
															</div>
															<span
																className={
																	styles["install-hero-name"]
																}
															>
																{characterLabel}
															</span>
														</div>
													)}
												</div>

												<div className={styles["install-mod-badges"]}>
													{categoryLabel ? (
														<span
															className={`mod-category-badge category-${categorySlug(categoryLabel)}`}
														>
															{categoryLabel}
														</span>
													) : null}
													<span
														className={[
															styles["install-format-badge"],
															isIoStore
																? styles.iostore
																: styles.classic,
														].join(" ")}
													>
														{isIoStore ? "IoStore" : "Classic"}
													</span>
													<span className={styles["install-size-badge"]}>
														{formatBytes(item.totalSizeBytes)}
													</span>
												</div>
											</div>
										</div>

										<div className={styles["install-mod-fields"]}>
											<label
												className="grid gap-1.5 text-xs font-bold text-muted-foreground"
												htmlFor={`mod-name-${item.id}`}
											>
												<span>Mod name</span>
												<input
													id={`mod-name-${item.id}`}
													type="text"
													className="min-w-0 px-3 py-2.5"
													value={config.modName}
													disabled={!isSelected || phase === "applying"}
													onChange={(event) =>
														handleConfigChange(item.id, {
															modName: event.target.value,
														})
													}
												/>
											</label>

											<label
												className="grid gap-1.5 text-xs font-bold text-muted-foreground"
												htmlFor={`mod-folder-${item.id}`}
											>
												<span>Destination folder</span>
												<Select
													value={
														config.destinationFolder === ""
															? LIBRARY_ROOT_OPTION
															: config.destinationFolder
													}
													disabled={!isSelected || phase === "applying"}
													onValueChange={(nextFolder) => {
														if (nextFolder === null) return;
														handleConfigChange(item.id, {
															destinationFolder:
																nextFolder === LIBRARY_ROOT_OPTION
																	? ""
																	: nextFolder,
														});
													}}
												>
													<SelectTrigger
														id={`mod-folder-${item.id}`}
														className="w-full"
													>
														<SelectValue>
															{config.destinationFolder === ""
																? "Library root"
																: config.destinationFolder}
														</SelectValue>
													</SelectTrigger>
													<SelectContent>
														<SelectItem value={LIBRARY_ROOT_OPTION}>
															Library root
														</SelectItem>
														{folders.map((folder) => (
															<SelectItem key={folder} value={folder}>
																{folder}
															</SelectItem>
														))}
													</SelectContent>
												</Select>
											</label>
										</div>

										<div className={styles["install-encryption-row"]}>
											<div>
												<p className={styles["install-encryption-label"]}>
													Encrypt on install
												</p>
												<p
													id={`mod-encryption-hint-${item.id}`}
													className={styles["install-encryption-hint"]}
												>
													{item.identity?.encrypted
														? "Already encrypted. The install preserves this state."
														: !item.canEncrypt
															? "Requires a complete IoStore bundle."
															: config.encrypt
																? "Encrypt this mod before install."
																: "Keep this mod's current encryption state."}
												</p>
											</div>
											<button
												type="button"
												role="switch"
												aria-checked={
													item.identity?.encrypted || config.encrypt
												}
												aria-label={`Encrypt ${config.modName} on install`}
												aria-describedby={`mod-encryption-hint-${item.id}`}
												className={styles["install-encryption-switch"]}
												disabled={
													!isSelected ||
													phase === "applying" ||
													!item.canEncrypt ||
													item.identity?.encrypted
												}
												onClick={() =>
													handleConfigChange(item.id, {
														encrypt: !config.encrypt,
													})
												}
											>
												<span
													className={styles["install-encryption-knob"]}
													aria-hidden="true"
												/>
											</button>
										</div>

										{error && (
											<p
												className="text-[0.8rem] leading-snug font-semibold text-destructive"
												role="alert"
											>
												{error}
											</p>
										)}

										{isSelected && item.unsupportedCompanionPak && (
											<div
												className={styles["install-warning-banner"]}
												role="status"
											>
												<TriangleAlert aria-hidden="true" />
												<div
													className={styles["install-collision-content"]}
												>
													<p>
														{config.modName} has unsupported companion
														PAK entries (chunknames / patched_files).{" "}
														{config.encrypt
															? "The encryption rebuild also removes these entries."
															: "Cratebug will rewrite only that .pak before install."}
													</p>
												</div>
											</div>
										)}

										{isSelected && item.issues && item.issues.length > 0 && (
											<div
												className={styles["install-collision-banner"]}
												role="alert"
											>
												<TriangleAlert aria-hidden="true" />
												<div
													className={styles["install-collision-content"]}
												>
													{item.issues.map((issue) => (
														<p key={issue.code}>{issue.message}</p>
													))}
													<p>
														This bundle cannot be installed as-is.
														Exclude it or resolve the underlying file
														first.
													</p>
												</div>
											</div>
										)}

										{hasCollision && (
											<div
												className={styles["install-collision-banner"]}
												role="alert"
											>
												<TriangleAlert aria-hidden="true" />
												<div
													className={styles["install-collision-content"]}
												>
													<p>
														{liveCollision.description ||
															item.collision?.description ||
															"A mod with this name already exists in the destination folder."}{" "}
														Change the mod name above to install
														alongside it instead.
													</p>
													<label
														className={
															styles["install-overwrite-checkbox"]
														}
													>
														<input
															type="checkbox"
															checked={config.overwrite}
															disabled={
																!isSelected || phase === "applying"
															}
															onChange={(event) =>
																handleConfigChange(item.id, {
																	overwrite: event.target.checked,
																})
															}
														/>
														<span>Overwrite existing mod</span>
													</label>
												</div>
											</div>
										)}

										{batchCollisions[item.id] && isSelected && (
											<div
												className={styles["install-collision-banner"]}
												role="alert"
											>
												<TriangleAlert aria-hidden="true" />
												<div
													className={styles["install-collision-content"]}
												>
													<p>{batchCollisions[item.id]}</p>
												</div>
											</div>
										)}

										<div className={styles["install-files-collapsible"]}>
											<button
												type="button"
												className={styles["install-files-toggle"]}
												onClick={() => toggleFileExpansion(item.id)}
												aria-expanded={isExpanded}
											>
												{isExpanded ? (
													<ChevronDown aria-hidden="true" />
												) : (
													<ChevronRight aria-hidden="true" />
												)}
												<span>
													{item.files.length === 1
														? "1 file"
														: `${item.files.length} files`}
												</span>
											</button>

											{isExpanded && (
												<ul
													className={[
														styles["install-files-list"],
														"scroll-y",
													].join(" ")}
												>
													{item.files.map((file) => (
														<li
															key={file}
															title={item.sourcePath || file}
														>
															{file}
														</li>
													))}
												</ul>
											)}
										</div>
									</div>
								);
							})}
						</div>

						<div className={styles["install-preview-footer"]}>
							{phase === "applying" && (
								<p className={styles["install-encryption-hint"]} role="status">
									{downloadProgress?.phase === "encrypting"
										? downloadProgress.message
										: "Install mods..."}
								</p>
							)}
							{unsupportedCompanionCount > 0 && (
								<p className={styles["install-footer-warning"]} role="status">
									{unsupportedCompanionCount === 1
										? "1 selected mod has unsupported companion PAK entries (chunknames / patched_files). Cratebug will remove these entries before install."
										: `${unsupportedCompanionCount} selected mods have unsupported companion PAK entries (chunknames / patched_files). Cratebug will remove these entries before install.`}
								</p>
							)}
							{blockingIssues && (
								<p className={styles["install-footer-error"]} role="alert">
									Exclude any mod with a staging issue before installing.
								</p>
							)}
							{hasBatchCollisions && (
								<p className={styles["install-footer-error"]} role="alert">
									Multiple selected mods target the same name and destination
									folder.
								</p>
							)}
							{unresolvedCollisions && (
								<p className={styles["install-footer-error"]} role="alert">
									Resolve all collisions before installing.
								</p>
							)}
							{selectedItems.length === 0 && (
								<p className={styles["install-footer-error"]} role="alert">
									Select at least 1 mod to install.
								</p>
							)}
							<div className="mt-[0.35rem] flex justify-end gap-[0.65rem]">
								<Button
									size="action"
									type="button"
									variant="secondary"
									disabled={phase === "applying"}
									onClick={() => void handleCancel()}
								>
									Cancel
								</Button>
								<Button
									size="action"
									type="button"
									disabled={!canInstall}
									onClick={() => void handleApply()}
								>
									{phase === "applying" ? (
										<>
											<Loader2
												className="spinning-loader"
												aria-hidden="true"
											/>
											<span>Installing...</span>
										</>
									) : (
										<>
											<PackagePlus aria-hidden="true" />
											<span>{buttonLabel}</span>
										</>
									)}
								</Button>
							</div>
						</div>
					</>
				)}
			</DialogContent>
		</Dialog>
	);
}
