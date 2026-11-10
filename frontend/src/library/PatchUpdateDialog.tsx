import {
	ChevronDown,
	CircleAlert,
	FolderOpen,
	Loader2,
	RefreshCw,
	TriangleAlert,
	X,
} from "lucide-react";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";

import {
	CancelPatchUpdate,
	CheckModsForPatch,
	OpenPatchOriginals,
	UpdateModsForPatch,
} from "../../wailsjs/go/main/PatchUpdateService";
import { mutation } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { formatWailsError } from "./installPresentation";
import {
	formatPatchUpdateFromBuilds,
	formatPatchUpdateProgress,
	groupPatchUpdatePlan,
	groupPatchUpdateResult,
	patchUpdateModLine,
	type PatchUpdatePlanGroup,
	type PatchUpdateProgressView,
	type PatchUpdateResultGroup,
	selectablePatchUpdateMods,
	summarizePatchUpdatePlan,
	summarizePatchUpdateResult,
} from "./patchUpdatePresentation";

import styles from "./PatchUpdateDialog.module.css";

type PatchUpdateDialogProps = {
	libraryRoot: string;
	// The mods to check. Empty means the whole library; the backend treats an
	// empty target list that way.
	targets: mutation.PatchUpdateTarget[];
	// Names the scope under the title, for example "1 mod" or "12 selected mods".
	scopeLabel: string;
	// The dialog asks for a rescan when it closed after changing mods.
	onClose: (reload: boolean) => void | Promise<void>;
};

type PatchUpdateDialogState =
	| { phase: "checking" }
	| { phase: "ready"; plan: mutation.PatchUpdatePlan }
	| {
			phase: "applying";
			plan: mutation.PatchUpdatePlan;
			progress: PatchUpdateProgressView | null;
	  }
	| {
			phase: "done";
			plan: mutation.PatchUpdatePlan;
			result: mutation.PatchUpdateResult;
			requested: number;
	  }
	| { phase: "error"; message: string; plan: mutation.PatchUpdatePlan | null };

// Checks the library for mods cooked before the current layout, lets the
// user choose which can be carried forward, then rewrites only those in place.
export function PatchUpdateDialog({
	libraryRoot,
	targets,
	scopeLabel,
	onClose,
}: PatchUpdateDialogProps) {
	const [state, setState] = useState<PatchUpdateDialogState>({
		phase: "checking",
	});
	const [selected, setSelected] = useState<Set<string>>(new Set());
	const [cancelling, setCancelling] = useState(false);
	const [originalsError, setOriginalsError] = useState("");
	const activeRef = useRef(true);
	const targetKey = targets.map((target) => target.entryID).join("\n");
	const checkKeyRef = useRef<string | null>(null);
	const checkGenerationRef = useRef(0);
	const unmountCancelRef = useRef<number | null>(null);
	const closeButtonRef = useRef<HTMLButtonElement>(null);

	useEffect(() => {
		closeButtonRef.current?.focus();
	}, []);

	// A dialog removed while a check or apply is still running must not leave
	// the backend working behind it. Deferring the cancel lets StrictMode's
	// simulated cleanup clear itself when the effect immediately remounts.
	useEffect(() => {
		if (unmountCancelRef.current !== null) {
			window.clearTimeout(unmountCancelRef.current);
			unmountCancelRef.current = null;
		}
		return () => {
			unmountCancelRef.current = window.setTimeout(() => {
				unmountCancelRef.current = null;
				void CancelPatchUpdate();
			}, 0);
		};
	}, []);

	useEffect(() => {
		const off = EventsOn("patchupdate:progress", (progress: PatchUpdateProgressView) => {
			setState((current) =>
				current.phase === "applying" ? { ...current, progress } : current,
			);
		});
		return off;
	}, []);

	const runCheck = useCallback(async () => {
		const generation = ++checkGenerationRef.current;
		setCancelling(false);
		setOriginalsError("");
		setState({ phase: "checking" });
		try {
			const plan = await CheckModsForPatch(libraryRoot, targets);
			if (!activeRef.current || generation !== checkGenerationRef.current) return;
			setSelected(new Set(selectablePatchUpdateMods(plan).map((mod) => mod.entryID)));
			setState({ phase: "ready", plan });
		} catch (error) {
			if (!activeRef.current || generation !== checkGenerationRef.current) return;
			setState({ phase: "error", message: formatWailsError(error), plan: null });
		}
	}, [libraryRoot, targets]);

	// StrictMode runs effects twice in development. Starting the check only
	// once per scope keeps the second pass from tripping the backend's
	// one-operation guard while the first check is still running.
	const checkKey = `${libraryRoot}\x00${targetKey}`;
	useEffect(() => {
		activeRef.current = true;
		if (checkKeyRef.current !== checkKey) {
			checkKeyRef.current = checkKey;
			void runCheck();
		}
		return () => {
			activeRef.current = false;
		};
	}, [checkKey, runCheck]);

	const close = useCallback(async () => {
		if (state.phase === "applying") return;
		const changed = state.phase === "done" && state.result.mods.some((mod) => mod.updated > 0);
		if (state.phase === "checking") {
			try {
				await CancelPatchUpdate();
			} catch {
				// The dialog is closing either way.
			}
		}
		await onClose(changed);
	}, [onClose, state]);

	async function cancelCheck() {
		setCancelling(true);
		await close();
	}

	async function cancelApply() {
		if (cancelling) return;
		setCancelling(true);
		try {
			await CancelPatchUpdate();
		} catch {
			// The apply promise reports the cancellation.
		}
	}

	async function apply() {
		if (state.phase !== "ready") return;
		const applyTargets = state.plan.mods
			.filter((mod) => selected.has(mod.entryID))
			.map(
				(mod) =>
					new mutation.PatchUpdateTarget({
						entryID: mod.entryID,
						displayName: mod.displayName,
					}),
			);
		if (applyTargets.length === 0) return;

		const plan = state.plan;
		setCancelling(false);
		setState({ phase: "applying", plan, progress: null });
		try {
			const result = await UpdateModsForPatch(libraryRoot, applyTargets);
			if (!activeRef.current) return;
			setState({ phase: "done", plan, result, requested: applyTargets.length });
		} catch (error) {
			if (!activeRef.current) return;
			setState({ phase: "error", message: formatWailsError(error), plan });
		}
	}

	function toggleEntry(entryID: string, checked: boolean) {
		setSelected((current) => {
			const next = new Set(current);
			if (checked) next.add(entryID);
			else next.delete(entryID);
			return next;
		});
	}

	const selectable = state.phase === "ready" ? selectablePatchUpdateMods(state.plan) : [];
	const selectedCount = selectable.filter((mod) => selected.has(mod.entryID)).length;
	const allSelected = selectable.length > 0 && selectedCount === selectable.length;

	function toggleAll() {
		setSelected(allSelected ? new Set() : new Set(selectable.map((mod) => mod.entryID)));
	}

	const title = dialogTitle(state);
	const errorPlan = state.phase === "error" ? state.plan : null;
	const progress = state.phase === "applying" ? state.progress : null;

	return (
		<Dialog
			open
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				if (nextOpen) return;
				if (state.phase === "applying") void cancelApply();
				else void close();
			}}
		>
			<DialogContent className="max-h-[min(calc(100vh_-_3.5rem),52rem)] overflow-hidden overscroll-contain sm:max-w-[56rem]">
				<div className={styles["patch-update-header"]}>
					<div>
						<p className="eyebrow">Game patch update · {scopeLabel}</p>
						<DialogTitle>{title}</DialogTitle>
					</div>
					{state.phase !== "applying" && (
						<Button
							ref={closeButtonRef}
							type="button"
							variant="secondary"
							size="icon"
							onClick={() => void close()}
							aria-label="Close"
							title="Close"
						>
							<X aria-hidden="true" />
						</Button>
					)}
				</div>

				{state.phase === "checking" && (
					<StatusState>
						<Loader2 className="spinning-loader" aria-hidden="true" />
						<p>Checking mods against the current game layout…</p>
					</StatusState>
				)}

				{state.phase === "error" && (
					<StatusState variant="error">
						<CircleAlert aria-hidden="true" />
						<p>{state.message}</p>
						<div className={styles["patch-status-actions"]}>
							{errorPlan ? (
								<Button
									type="button"
									variant="secondary"
									onClick={() => {
										if (errorPlan)
											setState({ phase: "ready", plan: errorPlan });
									}}
								>
									Back to selection
								</Button>
							) : (
								<Button
									type="button"
									variant="secondary"
									onClick={() => void runCheck()}
								>
									<RefreshCw aria-hidden="true" />
									Try again
								</Button>
							)}
							<Button type="button" variant="secondary" onClick={() => void close()}>
								Close
							</Button>
						</div>
					</StatusState>
				)}

				{(state.phase === "ready" || state.phase === "applying") && (
					<>
						<p className={styles["patch-summary"]}>
							{summarizePatchUpdatePlan(state.plan)}
						</p>
						<p className={styles["patch-game-path"]} title={state.plan.gamePaks}>
							Game: {state.plan.gamePaks}
						</p>
						{state.plan.mods.length === 0 ? (
							<StatusState>
								<p>No complete IoStore mods matched this check.</p>
							</StatusState>
						) : (
							<div className={styles["patch-update-list"]}>
								{groupPatchUpdatePlan(state.plan)
									.filter((group) => group.key !== "current")
									.map((group) => (
										<PatchUpdatePlanGroupSection
											key={group.key}
											group={group}
											applying={state.phase === "applying"}
											progress={progress}
											selected={selected}
											allSelected={allSelected}
											onToggle={toggleEntry}
											onToggleAll={toggleAll}
										/>
									))}
								{countCurrent(state.plan) > 0 && (
									<p className={styles["patch-current-note"]}>
										{countCurrent(state.plan)} already current and left
										untouched.
									</p>
								)}
							</div>
						)}
					</>
				)}

				{state.phase === "done" && (
					<>
						<p className={styles["patch-summary"]}>
							{summarizePatchUpdateResult(state.result)}
						</p>
						{state.result.cancelled && state.requested > state.result.mods.length && (
							<p className={styles["patch-footer-note"]} role="status">
								{state.requested - state.result.mods.length} selected{" "}
								{state.requested - state.result.mods.length === 1
									? "mod was"
									: "mods were"}{" "}
								not processed.
							</p>
						)}
						<div className={styles["patch-update-list"]}>
							{groupPatchUpdateResult(state.result)
								.filter((group) => group.key !== "current")
								.map((group) => (
									<PatchUpdateResultGroupSection key={group.key} group={group} />
								))}
						</div>
						{state.result.originalsDir && hasUpdated(state.result) && (
							<p className={styles["patch-originals-note"]}>
								Originals kept in <span>{state.result.originalsDir}</span>
							</p>
						)}
						{originalsError && (
							<p className={styles["patch-footer-error"]} role="alert">
								{originalsError}
							</p>
						)}
					</>
				)}

				<div className={styles["patch-update-footer"]}>
					{state.phase === "checking" && (
						<>
							<p className={styles["patch-footer-note"]}>
								Nothing is changed while checking.
							</p>
							<div className={styles["patch-footer-actions"]}>
								<Button
									type="button"
									variant="secondary"
									disabled={cancelling}
									onClick={() => void cancelCheck()}
								>
									{cancelling ? "Stopping…" : "Cancel"}
								</Button>
							</div>
						</>
					)}
					{state.phase === "ready" && (
						<>
							<p className={styles["patch-footer-note"]}>
								Updates run in place. Original files are kept first. Close Marvel
								Rivals before updating.
							</p>
							<div className={styles["patch-footer-actions"]}>
								<Button
									type="button"
									variant="secondary"
									onClick={() => void close()}
								>
									{selectable.length === 0 ? "Close" : "Cancel"}
								</Button>
								{selectable.length > 0 && (
									<Button
										type="button"
										size="action"
										disabled={selectedCount === 0}
										onClick={() => void apply()}
									>
										Update{" "}
										{selectedCount === selectable.length
											? modCount(selectedCount)
											: `${selectedCount} of ${modCount(selectable.length)}`}
									</Button>
								)}
							</div>
						</>
					)}
					{state.phase === "applying" && (
						<>
							<p className={styles["patch-footer-note"]} role="status">
								{progress
									? formatPatchUpdateProgress(progress)
									: "Starting the update…"}
							</p>
							<div className={styles["patch-footer-actions"]}>
								<Button
									type="button"
									variant="secondary"
									disabled={cancelling}
									onClick={() => void cancelApply()}
								>
									{cancelling ? "Cancelling…" : "Cancel update"}
								</Button>
							</div>
						</>
					)}
					{state.phase === "done" && (
						<>
							<p className={styles["patch-footer-note"]}>
								{state.result.cancelled
									? "The mods already updated stay updated."
									: "Only the selected mods were touched."}
							</p>
							<div className={styles["patch-footer-actions"]}>
								{hasUpdated(state.result) && (
									<Button
										type="button"
										variant="secondary"
										onClick={() => {
											setOriginalsError("");
											void OpenPatchOriginals(state.result.gameBuild).catch(
												(error: unknown) =>
													setOriginalsError(formatWailsError(error)),
											);
										}}
									>
										<FolderOpen aria-hidden="true" />
										Open originals
									</Button>
								)}
								<Button type="button" onClick={() => void close()}>
									Close
								</Button>
							</div>
						</>
					)}
				</div>
			</DialogContent>
		</Dialog>
	);
}

function StatusState({
	children,
	variant = "normal",
}: {
	children: ReactNode;
	variant?: "normal" | "error";
}) {
	return (
		<div
			className={[styles["patch-status-state"], variant === "error" ? styles.error : ""]
				.filter(Boolean)
				.join(" ")}
		>
			{children}
		</div>
	);
}

function PatchUpdatePlanGroupSection({
	group,
	applying,
	progress,
	selected,
	allSelected,
	onToggle,
	onToggleAll,
}: {
	group: PatchUpdatePlanGroup;
	applying: boolean;
	progress: PatchUpdateProgressView | null;
	selected: Set<string>;
	allSelected: boolean;
	onToggle: (entryID: string, checked: boolean) => void;
	onToggleAll: () => void;
}) {
	return (
		<section className={styles["patch-group"]}>
			<div className={styles["patch-group-header"]}>
				<div>
					<h3>
						{group.title} ({group.mods.length})
					</h3>
					<p>{group.description}</p>
				</div>
				{group.selectable && !applying && (
					<Button type="button" variant="ghost" size="sm" onClick={onToggleAll}>
						{allSelected ? "Clear" : "Select all"}
					</Button>
				)}
			</div>
			<div className={styles["patch-group-list"]}>
				{group.mods.map((mod) => (
					<PatchUpdateModCard
						key={mod.entryID}
						mod={mod}
						phase="plan"
						selectable={group.selectable}
						selected={selected.has(mod.entryID)}
						disabled={applying}
						active={applying && progress?.entryID === mod.entryID}
						onToggle={onToggle}
					/>
				))}
			</div>
		</section>
	);
}

function PatchUpdateResultGroupSection({ group }: { group: PatchUpdateResultGroup }) {
	return (
		<section className={styles["patch-group"]}>
			<div className={styles["patch-group-header"]}>
				<div>
					<h3>
						{group.title} ({group.mods.length})
					</h3>
					<p>{group.description}</p>
				</div>
			</div>
			<div className={styles["patch-group-list"]}>
				{group.mods.map((mod) => (
					<PatchUpdateModCard
						key={mod.entryID}
						mod={mod}
						phase="result"
						selectable={false}
						selected={false}
						disabled
						active={false}
					/>
				))}
			</div>
		</section>
	);
}

function PatchUpdateModCard({
	mod,
	phase,
	selectable,
	selected,
	disabled,
	active,
	onToggle,
}: {
	mod: mutation.PatchUpdateMod;
	phase: "plan" | "result";
	selectable: boolean;
	selected: boolean;
	disabled: boolean;
	active: boolean;
	onToggle?: (entryID: string, checked: boolean) => void;
}) {
	const fromBuilds = formatPatchUpdateFromBuilds(mod.fromBuilds);
	const line = patchUpdateModLine(mod, phase);
	const assets = mod.assets ?? [];

	return (
		<article
			className={[
				styles["patch-mod-card"],
				!selectable && phase === "plan" ? styles.locked : "",
				selectable && !selected ? styles.unselected : "",
			]
				.filter(Boolean)
				.join(" ")}
		>
			<div className={styles["patch-mod-header"]}>
				<div className={styles["patch-mod-heading"]}>
					{selectable && (
						<label className={styles["patch-mod-select"]}>
							<input
								type="checkbox"
								checked={selected}
								disabled={disabled}
								onChange={(event) => onToggle?.(mod.entryID, event.target.checked)}
								aria-label={`Update ${mod.displayName}`}
							/>
						</label>
					)}
					<div>
						<h4>{mod.displayName}</h4>
						<p>
							{mod.relativeFolder || "Library root"}
							{!mod.enabled ? " · Disabled" : ""}
						</p>
					</div>
				</div>
				<div className={styles["patch-mod-badges"]}>
					{active && <Loader2 className="spinning-loader" aria-hidden="true" />}
					{fromBuilds && <span className={styles["patch-mod-build"]}>{fromBuilds}</span>}
					{phase === "plan" && mod.status === "partial" && (
						<span className={styles["patch-mod-warning"]} title={line}>
							<TriangleAlert aria-hidden="true" />
							Partly updatable
						</span>
					)}
				</div>
			</div>
			<p className={styles["patch-mod-line"]}>{line}</p>
			{assets.length > 0 && (
				<details className={styles["patch-mod-details"]}>
					<summary>
						<ChevronDown aria-hidden="true" />
						{assets.length === 1 ? "1 file" : `${assets.length} files`}
					</summary>
					<ul>
						{assets.map((asset) => (
							<li key={asset.path}>
								<span className={styles["patch-asset-path"]}>{asset.path}</span>
								<span className={styles["patch-asset-note"]}>
									{asset.outcome === "updated" && asset.fromBuild
										? `from build ${asset.fromBuild}`
										: (asset.reason ?? asset.outcome)}
								</span>
							</li>
						))}
					</ul>
				</details>
			)}
		</article>
	);
}

function dialogTitle(state: PatchUpdateDialogState): string {
	switch (state.phase) {
		case "checking":
			return "Checking mods for a game patch";
		case "ready":
			return `Update mods for build ${state.plan.gameBuild}`;
		case "applying":
			return "Updating mods";
		case "done":
			return state.result.cancelled ? "Patch update cancelled" : "Patch update complete";
		case "error":
			return "Patch update";
	}
}

function countCurrent(plan: mutation.PatchUpdatePlan): number {
	return plan.mods.filter((mod) => mod.status === "current").length;
}

function hasUpdated(result: mutation.PatchUpdateResult): boolean {
	return result.mods.some((mod) => mod.updated > 0);
}

function modCount(count: number): string {
	return `${count} ${count === 1 ? "mod" : "mods"}`;
}
