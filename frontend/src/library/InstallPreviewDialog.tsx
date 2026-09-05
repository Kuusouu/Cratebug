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
import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import {
	ApplyInstall,
	CancelInstall,
	InstallFromURL,
	PrepareInstall,
} from "../../wailsjs/go/main/App";
import { type discovery, install } from "../../wailsjs/go/models";
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
	type ModConfig,
	validateInstallModName,
} from "./installPresentation";
import { useDialogFocusTrap } from "./useDialogFocusTrap";

// A locally-selected/dropped set of files, or a single remote URL to
// download first -- InstallFromURL handles the download and stages the
// result through the exact same pipeline PrepareInstall uses for local files.
export type InstallSource = { kind: "files"; paths: string[] } | { kind: "url"; url: string };

export type InstallPreviewDialogProps = {
	modRoot: string;
	source: InstallSource;
	defaultFolder: string;
	folders: string[];
	libraryEntries?: readonly discovery.Entry[];
	onDone: (result: install.ApplyResult) => void;
	onCancel: () => void;
};

type DialogPhase = "preparing" | "ready" | "applying" | "error";

export function InstallPreviewDialog({
	modRoot,
	source,
	defaultFolder,
	folders,
	libraryEntries = [],
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
	const titleId = useId();

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	// Cancel session on escape or background close
	const handleCancel = useCallback(async () => {
		const session = sessionIdRef.current;
		if (session) {
			try {
				await CancelInstall(session);
			} catch {
				// Best-effort cleanup
			}
		}
		onCancel();
	}, [onCancel]);

	const dialogRef = useDialogFocusTrap<HTMLElement>(() => {
		if (phase !== "applying") {
			void handleCancel();
		}
	});

	// Stage files and generate preview on mount
	useEffect(() => {
		let isMounted = true;

		async function prepare() {
			setPhase("preparing");
			setErrorMessage("");
			try {
				const result =
					source.kind === "url"
						? await InstallFromURL(modRoot, source.url, defaultFolder)
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

	const items = previewResult?.items ?? [];

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
		<div className="mutation-dialog-backdrop">
			<section
				ref={dialogRef}
				className={["mutation-dialog", styles["install-preview-dialog"]].join(" ")}
				aria-labelledby={titleId}
				aria-modal="true"
				role="dialog"
			>
				<div className={styles["install-preview-header"]}>
					<div>
						<p className="eyebrow">Mod installation</p>
						<h2 id={titleId}>
							{phase === "preparing"
								? "Inspecting files..."
								: items.length === 1
									? "Install 1 mod"
									: `Install ${items.length} mods`}
						</h2>
					</div>
					{phase !== "applying" && (
						<button
							ref={closeButtonRef}
							type="button"
							className="icon-button"
							onClick={() => void handleCancel()}
							aria-label="Close"
							title="Cancel installation"
						>
							<X aria-hidden="true" />
						</button>
					)}
				</div>

				{phase === "preparing" && (
					<div className={styles["install-preview-status-state"]}>
						<Loader2 className="spinning-loader" aria-hidden="true" />
						<p>
							{source.kind === "url"
								? "Downloading, extracting, and discovering mod bundles..."
								: "Extracting archives and discovering mod bundles..."}
						</p>
					</div>
				)}

				{phase === "error" && (
					<div
						className={[styles["install-preview-status-state"], styles.error].join(" ")}
					>
						<CircleAlert aria-hidden="true" />
						<p>{errorMessage || "An error occurred during installation."}</p>
						<div className="mutation-dialog-actions">
							<button
								type="button"
								className="quiet-button"
								onClick={() => void handleCancel()}
							>
								Close
							</button>
						</div>
					</div>
				)}

				{(phase === "ready" || phase === "applying") && items.length === 0 && (
					<div className={styles["install-preview-status-state"]}>
						<Package aria-hidden="true" />
						<p>No installable mod files were found in the selected files.</p>
						<div className="mutation-dialog-actions">
							<button
								type="button"
								className="quiet-button"
								onClick={() => void handleCancel()}
							>
								Close
							</button>
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
												className="mutation-dialog-field"
												htmlFor={`mod-name-${item.id}`}
											>
												<span>Mod name</span>
												<input
													id={`mod-name-${item.id}`}
													type="text"
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
												className="mutation-dialog-field"
												htmlFor={`mod-folder-${item.id}`}
											>
												<span>Destination folder</span>
												<select
													id={`mod-folder-${item.id}`}
													value={config.destinationFolder}
													disabled={!isSelected || phase === "applying"}
													onChange={(event) =>
														handleConfigChange(item.id, {
															destinationFolder: event.target.value,
														})
													}
												>
													<option value="">Library root</option>
													{folders.map((folder) => (
														<option key={folder} value={folder}>
															{folder}
														</option>
													))}
												</select>
											</label>
										</div>

										{error && (
											<p className="mutation-dialog-error" role="alert">
												{error}
											</p>
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
							{blockingIssues && (
								<p className={styles["install-footer-warning"]} role="alert">
									Exclude any mod with a staging issue before installing.
								</p>
							)}
							{hasBatchCollisions && (
								<p className={styles["install-footer-warning"]} role="alert">
									Multiple selected mods target the same name and destination
									folder.
								</p>
							)}
							{unresolvedCollisions && (
								<p className={styles["install-footer-warning"]} role="alert">
									Resolve all collisions before installing.
								</p>
							)}
							{selectedItems.length === 0 && (
								<p className={styles["install-footer-warning"]} role="alert">
									Select at least 1 mod to install.
								</p>
							)}
							<div className="mutation-dialog-actions">
								<button
									type="button"
									className="quiet-button"
									disabled={phase === "applying"}
									onClick={() => void handleCancel()}
								>
									Cancel
								</button>
								<button
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
								</button>
							</div>
						</div>
					</>
				)}
			</section>
		</div>
	);
}
